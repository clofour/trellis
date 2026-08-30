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

// Job spec types (mirrors the orchestrator spec package)

export interface JobSpec {
  name: string;
  namespace?: string;
  task_groups: TaskGroupSpec[];
}

export interface TaskGroupSpec {
  name: string;
  count: number;
  api_access?: boolean;
  tasks: TaskSpec[];
}

export interface TaskSpec {
  name: string;
  image: string;
  env?: Record<string, string>;
  ports?: PortSpec[];
  resources?: ResourcesSpec;
  health_check?: HealthCheckSpec;
}

export interface PortSpec {
  host_port: number;
  container_port: number;
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
}
