package service

import "context"

type ServiceRegistry interface {
	Register(ctx context.Context)
	Deregister(ctx context.Context)
}
