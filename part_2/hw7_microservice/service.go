package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"

	"google.golang.org/grpc"
)

var aclStorage map[string]json.RawMessage

type service struct {
	m                    *sync.RWMutex
	incomingLogsCh       chan *logMsg
	closeListenersCh     chan struct{}
	listeners            []*listener
	aclStorage           map[string][]string
	statListeners        []*statListener
	incomingStatCh       chan *statMsg
	closeStatListenersCh chan struct{}
}

type logMsg struct {
	methodName   string
	consumerName string
}

type listener struct {
	logsCh  chan *logMsg
	closeCh chan struct{}
}

type statMsg struct {
	methodName   string
	consumerName string
}

type statListener struct {
	statCh  chan *statMsg
	closeCh chan struct{}
}

func StartMyMicroservice(ctx context.Context, addr, acl string) error {
	aclParsed, err := parseACL(acl)
	if err != nil {
		return err
	}

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		panic(fmt.Sprintf("can not start the service. %s", err.Error()))
	}

	service := &service{
		m:                    &sync.RWMutex{},
		incomingLogsCh:       make(chan *logMsg, 0),
		listeners:            make([]*listener, 0),
		aclStorage:           aclParsed,
		closeListenersCh:     make(chan struct{}),
		statListeners:        make([]*statListener, 0),
		incomingStatCh:       make(chan *statMsg, 0),
		closeStatListenersCh: make(chan struct{}),
	}

	go service.logsSender()
	go service.statsSender()

	opts := []grpc.ServerOption{grpc.UnaryInterceptor(service.unaryInterceptor),
		grpc.StreamInterceptor(service.streamInterceptor)}

	srv := grpc.NewServer(opts...)

	RegisterBizServer(srv, service)
	RegisterAdminServer(srv, service)

	go func() {
		select {
		case <-ctx.Done():
			service.closeListenersCh <- struct{}{}

			service.closeStatListenersCh <- struct{}{}

			srv.Stop()
			return
		}
	}()

	go func() {
		err := srv.Serve(lis)
		if err != nil {
			panic(err)
		}
		return
	}()

	return nil
}

func (s *service) unaryInterceptor(ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler) (interface{}, error) {
	consumer, err := getConsumerNameFromContext(ctx)
	if err != nil {
		return nil, err
	}

	err = s.checkBizPermission(consumer, info.FullMethod)
	if err != nil {
		return nil, err
	}

	logMsg := logMsg{
		consumerName: consumer,
		methodName:   info.FullMethod,
	}

	s.incomingLogsCh <- &logMsg

	statMsg := statMsg{
		consumerName: consumer,
		methodName:   info.FullMethod,
	}

	s.incomingStatCh <- &statMsg

	h, err := handler(ctx, req)
	return h, err
}

func (s *service) streamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {

	consumer, err := getConsumerNameFromContext(ss.Context())
	if err != nil {
		return err
	}

	err = s.checkBizPermission(consumer, info.FullMethod)
	if err != nil {
		return err
	}

	if info.FullMethod == "/main.Admin/Logging" {
		msg := logMsg{
			consumerName: consumer,
			methodName:   info.FullMethod,
		}
		s.m.RLock()
		for _, l := range s.listeners {
			l.logsCh <- &msg
		}
		s.m.RUnlock()

	} else {
		msg := statMsg{
			consumerName: consumer,
			methodName:   info.FullMethod,
		}

		s.m.RLock()
		for _, l := range s.statListeners {
			l.statCh <- &msg
		}
		s.m.RUnlock()

	}

	return handler(srv, ss)
}

func (service) mustEmbedUnimplementedBizServer() {}

func (service) mustEmbedUnimplementedAdminServer() {}
