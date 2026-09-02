package spec

// GroupUsesWireGuard reports whether any task in a group requests the namespace WireGuard network.
func GroupUsesWireGuard(group *TaskGroupSpec) bool {
	if group == nil {
		return false
	}
	for i := range group.Tasks {
		if group.Tasks[i].Networking != nil && group.Tasks[i].Networking.Mode == TaskNetworkWireGuard {
			return true
		}
	}
	return false
}
