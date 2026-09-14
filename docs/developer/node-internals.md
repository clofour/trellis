# Runtime, networking, storage, and secrets

## Agent convergence

The server submits one allocation containing all tasks in a group. The agent validates epoch/generation, serializes local reconciliation, prepares network/volumes/secrets/ports, and asks the selected runtime to create/start containers. Runtime labels retain allocation identity and generation so an agent can recover ownership after restart. Delete operations are likewise generation-aware. The agent API is an internal control surface, not a user API.

## Runtime abstraction

`ContainerRuntime` provides create/start/stop/remove/status/list/log operations. `ManagedRuntime` adds lifecycle management. The containerd implementation creates OCI containers, applies task environment/resources/mounts/ports/runtime selection, and streams logs. `runc` is normal OCI execution; `runsc` requires a matching containerd runtime installation. `InjectedRuntime` and its fault file exist for deterministic tests and must not host real workloads.

## Ports and health

A host port of zero requests allocation by the node port manager; a nonzero port reserves that exact host port. Reported mappings feed status and service discovery. Health checks run on configured intervals/timeouts and require a threshold of failures/successes before state changes. Script probes execute the supplied command in the task context; HTTP/TCP probes use the configured port.

## Volumes

A volume is identified for scheduling by `(namespace, name)`. The scheduler reserves an unseen identity to the node selected for its first allocation; once the node realizes the mount, it persists that registration locally and advertises `namespace/name` on subsequent registration/heartbeat traffic. Later allocations using the same identity may run only on that node. Conflicting advertisements are treated as unschedulable instead of choosing an arbitrary copy.

`host_path` controls only the node-side backing path. `@/relative/path` resolves below `<data-dir>/volumes/namespaces/<namespace>/` and the volume manager creates the directory. A clean absolute path is used verbatim and must already exist. Absolute paths deliberately bypass filesystem-level namespace separation, although the logical registration remains namespace-scoped. `container_path` is the absolute destination passed to the runtime; `read_only` changes the OCI mount.

Volume registrations and their data outlive individual allocations. Changing `host_path` for an existing name changes its backing path on the already owning node; Trellis does not move bytes. Volume registration is locality metadata, not replication, snapshots, backup, migration, or distributed storage.

## Networking

Host mode bypasses allocation isolation. Isolated allocations can receive an automatically derived namespace address and WireGuard peer plan when the job enables WireGuard. The leader deterministically derives namespace subnets and node peer information from the configured cluster pool. The node manager materializes interfaces/routes/peers; disabled networking returns an explicit capability error. Trellis DNS answers catalog lookups over UDP.

## Secrets and API access

The secret store accepts at most 64 KiB, encrypts values with the configured 32-byte key, stores versioned ciphertext and key ID in durable replicated state, and exposes metadata without plaintext. Secret delivery occurs during allocation start. Environment targets become container environment; file targets are materialized only below `/run/trellis-secrets/` with restricted modes and cleaned up with the allocation.

API access is a task-group mode. With `api_access: namespace`, the control plane obtains or creates a persistent bearer token for the allocation job's own namespace. With `api_access: cluster`, it injects the cluster administrator credential already used by the server for authenticated control-plane/agent operation. Both modes inject `TRELLIS_ADDR`, `TRELLIS_TOKEN`, and `TRELLIS_NAMESPACE`, where the namespace variable is always the job namespace; `TRELLIS_CA_CERT` is added when the cluster CA is available.

Namespace authorization is carried in the persisted token scope, not trusted from the environment or request header, so changing `TRELLIS_NAMESPACE` cannot escape the job namespace. Cluster-token requests receive administrator context, so the default namespace is only a client convenience for that mode. Every task in the group receives the selected credential, making the task-group boundary a security boundary as well as a lifecycle boundary.
