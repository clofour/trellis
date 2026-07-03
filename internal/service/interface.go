package service

import "context"

type ServiceRegistry interface {
	Register(ctx context.Context, ID string, name string, addr string, port int)
	Deregister(ctx context.Context)
}
