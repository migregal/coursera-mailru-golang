package main

import "context"

func (s *service) Check(ctx context.Context, n *Nothing) (*Nothing, error) {
	return &Nothing{}, nil
}

func (s *service) Add(ctx context.Context, n *Nothing) (*Nothing, error) {
	return &Nothing{}, nil
}

func (s *service) Test(ctx context.Context, n *Nothing) (*Nothing, error) {
	return &Nothing{}, nil
}
