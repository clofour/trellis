package service

import "context"

type ServiceRegistry interface {
	Register(ctx context.Context, ID string, name string, addr string, port int) error
	Deregister(ctx context.Context, ID string) error
}
