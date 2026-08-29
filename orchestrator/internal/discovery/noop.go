package discovery

import "context"

type NoopRegistry struct{}

func (NoopRegistry) Register(_ context.Context, _, _, _ string, _ int) error { return nil }
func (NoopRegistry) Deregister(_ context.Context, _ string) error            { return nil }

var _ ServiceRegistry = NoopRegistry{}
