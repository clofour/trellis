export type NodeStatus = "healthy" | "unhealthy" | "draining";

export interface Node {
  id: string;
  host: string;
  port: number;
  status: NodeStatus;
  last_heartbeat: string;
  cpu: number;
  memory: number;
  labels?: Record<string, string>;
  volumes?: string[];
}

export interface PortMapping {
  host_port: number;
  container_port: number;
}

export type AllocationStatus =
  | AllocationPhase
  | "pending"
  | "healthy"
  | "unhealthy";
export type AllocationPhase =
  | "placed"
  | "starting"
  | "running"
  | "stopping"
  | "stopped"
  | "failed"
  | "lost";
export type AllocationHealth = "unknown" | "healthy" | "unhealthy";

export interface Allocation {
  id: string;
  job?: string;
  group: string;
  task: string;
  namespace?: string;
  node_id: string;
  labels?: Record<string, string>;
  address?: string;
  ports?: PortMapping[];
  status: AllocationStatus;
  phase: AllocationPhase;
  health: AllocationHealth;
  draining?: boolean;
  generation: number;
  job_revision: number;
  created_at?: string;
  last_transition_at?: string;
  reason?: string;
  message?: string;
  attempt?: number;
  next_retry_at?: string | null;
}

export interface AllocationEvent {
  phase: AllocationPhase;
  reason?: string;
  message?: string;
  at: string;
}

export interface Job {
  name: string;
  revision: number;
  desired: number;
  running: number;
  healthy: number;
  allocations: Allocation[] | null;
  spec?: JobSpec;
}

export interface SecretMetadata {
  namespace: string;
  name: string;
  version: number;
  created_at: string;
  updated_at: string;
  ciphertext_size: number;
  key_id: string;
}

// Job spec types. Keep these aligned with orchestrator/internal/spec/types.go.
// time.Duration is encoded as nanoseconds by Go's JSON encoder; the dashboard
// editor also accepts manifest-style strings such as "10s" and normalizes them
// before submission.
export type DurationValue = number | string;

export interface JobSpec {
  name: string;
  namespace: string;
  network?: NetworkSpec;
  task_groups: TaskGroupSpec[];
}

export interface NetworkSpec {
  wireguard: boolean;
}

export type Runtime = "" | "runc" | "runsc";
export type NetworkMode = "" | "host";
export type UpdateStrategy = "" | "recreate" | "rolling";

export interface TaskGroupSpec {
  name: string;
  count: number;
  runtime?: Runtime;
  tasks: TaskSpec[];
  labels?: Record<string, string>;
  network_mode?: NetworkMode;
  api_access?: boolean;
  restart?: RestartPolicySpec;
  constraints?: ConstraintSpec[];
  update?: UpdateSpec;
}

export interface ConstraintSpec {
  attribute: string;
  value: string;
}

export interface RestartPolicySpec {
  max_restarts: number;
  window: DurationValue;
}

export interface UpdateSpec {
  strategy: UpdateStrategy;
  max_parallel?: number;
}

export interface TaskSpec {
  name: string;
  image: string;
  env?: Record<string, string>;
  ports?: PortSpec[];
  volumes?: VolumeSpec[];
  resources?: ResourcesSpec;
  health_check?: HealthCheckSpec;
  secrets?: SecretRefSpec[];
}

export interface PortSpec {
  host_port: number;
  container_port: number;
}

export interface VolumeSpec {
  name: string;
  path: string;
  host_volume?: string;
  read_only?: boolean;
}

export type SecretTarget = "env" | "file";

export interface SecretRefSpec {
  name: string;
  target: SecretTarget;
  env?: string;
  path?: string;
  mode?: number;
}

export interface ResourcesSpec {
  cpu: number;
  memory: number;
}

export interface HealthCheckSpec {
  type: "http" | "tcp" | "script";
  port: number;
  path?: string;
  command?: string[];
  interval?: DurationValue;
  timeout?: DurationValue;
  threshold?: number;
}
