package network

import "context"

type Attachment struct {
	AllocationID string
	Tenant       string
	Network      string
	Namespace    string
	HostVeth     string
	Address      string
	LeasePath    string
}

type Manager interface {
	Attach(context.Context, string, string, string) (*Attachment, error)
	Detach(context.Context, *Attachment) error
}

type DisabledManager struct{}

func (DisabledManager) Attach(context.Context, string, string, string) (*Attachment, error) {
	return nil, ErrDisabled
}
func (DisabledManager) Detach(context.Context, *Attachment) error { return nil }

type disabledError struct{}

func (disabledError) Error() string { return "WireGuard networking is not configured on this node" }

var ErrDisabled error = disabledError{}
