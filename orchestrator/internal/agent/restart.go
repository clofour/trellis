package agent

import "github.com/clofour/trellis/internal/runtime"

// RestartController is retained as a compatibility alias while callers move to
// the allocation reconciler terminology. Restart policy is now implemented by
// AllocationReconciler alongside other local lifecycle decisions.
type RestartController = AllocationReconciler

// RestartSubscriber receives allocation reconciliation changes.
type RestartSubscriber = AllocationReconcileSubscriber

// NewRestartController creates an allocation restart reconciler.
func NewRestartController(runtime runtime.ContainerRuntime, subscriber RestartSubscriber) *AllocationReconciler {
	return NewAllocationReconciler(runtime, subscriber)
}
