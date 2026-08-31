# Runtime, networking, storage, and secrets

## Agent convergence

The server submits one allocation containing all tasks in a group. The agent validates epoch/generation, serializes local reconciliation, prepares network/volumes/secrets/ports, and asks the selected runtime to create/start containers. Runtime labels retain allocation identity and generation so an agent can recover ownership after restart. Delete operations are likewise generation-aware. The agent API is an internal control surface, not a user API.

## Runtime abstraction

`ContainerRuntime` provides create/start/stop/remove/status/list/log operations. `ManagedRuntime` adds lifecycle management. The containerd implementation creates OCI containers, applies task environment/resources/mounts/ports/runtime selection, and streams logs. `runc` is normal OCI execution; `runsc` requires a matching containerd runtime installation. `InjectedRuntime` and its fault file exist for deterministic tests and must not host real workloads.

## Ports and health

A host port of zero requests allocation by the node port manager; a nonzero port reserves that exact host port. Reported mappings feed status and service discovery. Health checks run on configured intervals/timeouts and require a threshold of failures/successes before state changes. Script probes execute the supplied command in the task context; HTTP/TCP probes use the configured port.

## Volumes

The volume manager creates allocation-local paths below the data directory and mounts them at absolute container paths. `read_only` changes the OCI mount. `host_volume` is a scheduling capability name: nodes advertise available names and the server restricts placement accordingly. It is node-local persistence, not replication or a distributed volume. Operators must provision, back up, and consistently map these names.

## Networking

Host mode bypasses allocation isolation. Isolated allocations can receive an automatically derived namespace address and WireGuard peer plan when the job enables WireGuard. The leader deterministically derives namespace subnets and node peer information from the configured cluster pool. The node manager materializes interfaces/routes/peers; disabled networking returns an explicit capability error. Trellis DNS answers catalog lookups over UDP.

## Secrets and API access

The secret store accepts at most 64 KiB, encrypts values with the configured 32-byte key, stores versioned ciphertext and key ID in durable replicated state, and exposes metadata without plaintext. Secret delivery occurs during allocation start. Environment targets become container environment; file targets are materialized only below `/run/trellis-secrets/` with restricted modes and cleaned up with the allocation.

With `api_access: true`, the control plane obtains or creates a persistent namespace bearer token and injects `TRELLIS_ADDR`, `TRELLIS_TOKEN`, and `TRELLIS_NAMESPACE`. This is a privilege boundary: any code in that group can invoke every endpoint allowed by that token. Cluster-token requests receive administrator context; namespace tokens are checked against namespace scope.
