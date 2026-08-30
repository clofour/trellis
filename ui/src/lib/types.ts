export type NodeStatus = "healthy" | "unhealthy" | "draining";

export interface Node {
  id: string;
  host: string;
  port: number;
  status: NodeStatus;
  last_heartbeat: string;
  cpu: number;
  memory: number;
}

export type AllocationStatus = AllocationPhase | "pending" | "healthy" | "unhealthy";
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
  group: string;
  task: string;
  node_id: string;
  status: AllocationStatus;
  phase: AllocationPhase;
  health: AllocationHealth;
  generation: number;
  job_revision: number;
  reason?: string;
  message?: string;
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

// Job spec types mirror orchestrator/internal/spec. Keep the transport schema
// here; form-specific editable state belongs in job-form-model.ts.

export type Runtime = "" | "runc" | "runsc";
export type NetworkMode = "" | "host";
export type HealthCheckType = "http" | "tcp" | "script";

export interface JobSpec {
  name: string;
  namespace?: string;
  network?: NetworkSpec;
  task_groups: TaskGroupSpec[];
}

export interface NetworkSpec {
  wireguard: boolean;
}

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
}

export interface ConstraintSpec {
  attribute: string;
  value: string;
}

export interface RestartPolicySpec {
  max_restarts: number;
  window: number;
}

export interface TaskSpec {
  name: string;
  image: string;
  env?: Record<string, string>;
  ports?: PortSpec[];
  volumes?: VolumeSpec[];
  resources?: ResourcesSpec;
  health_check?: HealthCheckSpec;
}

export interface PortSpec {
  host_port: number;
  container_port: number;
}

export interface VolumeSpec {
  name: string;
  path: string;
}

export interface ResourcesSpec {
  cpu: number;
  memory: number;
}

export interface HealthCheckSpec {
  type: HealthCheckType;
  port: number;
  path?: string;
  command?: string[];
  interval?: number;
  timeout?: number;
  threshold?: number;
}
