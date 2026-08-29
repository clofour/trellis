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

export type AllocationStatus = "pending" | "healthy" | "unhealthy";

export interface Allocation {
  id: string;
  group: string;
  task: string;
  node_id: string;
  status: AllocationStatus;
}

export interface Job {
  name: string;
  revision: number;
  desired: number;
  running: number;
  healthy: number;
  allocations: Allocation[] | null;
}

export interface PortMapping {
  host_port: number;
  container_port: number;
}

export interface Service {
  id: string;
  job: string;
  group: string;
  namespace: string;
  labels?: Record<string, string>;
  address: string;
  ports?: PortMapping[];
  status: string;
}
