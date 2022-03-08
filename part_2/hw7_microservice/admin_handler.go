package main

import (
	"fmt"
	"time"
)

func (s *service) Logging(nothing *Nothing, srv Admin_LoggingServer) error {

	listener := listener{
		logsCh:  make(chan *logMsg),
		closeCh: make(chan struct{}),
	}
	s.addListener(&listener)

	for {
		select {
		case logMsg := <-listener.logsCh:
			event := &Event{
				Consumer: logMsg.consumerName,
				Method:   logMsg.methodName,
				Host:     "127.0.0.1:8083",
			}
			srv.Send(event)

		case <-listener.closeCh:
			return nil
		}
	}
}

func (s *service) Statistics(interval *StatInterval, srv Admin_StatisticsServer) error {

	closeCh := make(chan struct{})

	ticker := time.NewTicker(time.Second * time.Duration(interval.IntervalSeconds))

	sl := statListener{
		statCh:  make(chan *statMsg, 0),
		closeCh: make(chan struct{}, 0),
	}

	s.addStatListener(&sl)

	c := make(map[string]uint64)
	m := make(map[string]uint64)

	for {
		select {
		case <-ticker.C:
			statEvent := &Stat{
				Timestamp:  0,
				ByMethod:   m,
				ByConsumer: c,
			}

			srv.Send(statEvent)

			c = make(map[string]uint64)
			m = make(map[string]uint64)

		case statMsg := <-sl.statCh:
			_, ok := c[statMsg.consumerName]
			if !ok {
				c[statMsg.consumerName] = 1
			} else {
				c[statMsg.consumerName]++
			}

			_, ok = m[statMsg.methodName]
			if !ok {
				m[statMsg.methodName] = 1
			} else {
				m[statMsg.methodName]++
			}

		case <-closeCh:
			fmt.Println("CLOSED")
			return nil
		}
	}

	return nil
}
