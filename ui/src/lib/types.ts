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
}
