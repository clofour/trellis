import type {
  JobSpec,
  PortSpec,
  TaskGroupSpec,
  TaskSpec,
  HealthCheckSpec,
} from "@/lib/types";

// ── Internal form state types ────────────────────────────────────────

export interface EnvEntry {
  key: string;
  value: string;
}

export interface TaskForm {
  name: string;
  image: string;
  envEntries: EnvEntry[];
  cpuMillicores: string;
  memoryMB: string;
  ports: PortSpec[];
  healthEnabled: boolean;
  healthType: "http" | "tcp" | "exec";
  healthPort: string;
  healthPath: string;
  healthCommand: string;
}

export interface GroupForm {
  name: string;
  count: string;
  apiAccess: boolean;
  tasks: TaskForm[];
}

export interface FormState {
  name: string;
  groups: GroupForm[];
}

// ── Conversions ──────────────────────────────────────────────────────

export function specToForm(spec: JobSpec): FormState {
  return {
    name: spec.name,
    groups: spec.task_groups.map((g) => ({
      name: g.name,
      count: String(g.count),
      apiAccess: g.api_access ?? false,
      tasks: g.tasks.map((t) => ({
        name: t.name,
        image: t.image,
        envEntries: Object.entries(t.env ?? {}).map(([key, value]) => ({
          key,
          value,
        })),
        cpuMillicores: t.resources ? String(t.resources.cpu) : "",
        memoryMB: t.resources
          ? String(Math.round(t.resources.memory / (1024 * 1024)))
          : "",
        ports: t.ports ?? [],
        healthEnabled: !!t.health_check,
        healthType: t.health_check?.type ?? "http",
        healthPort: t.health_check ? String(t.health_check.port) : "",
        healthPath: t.health_check?.path ?? "/",
        healthCommand: t.health_check?.command?.join(" ") ?? "",
      })),
    })),
  };
}

export function formToSpec(form: FormState): JobSpec {
  return {
    name: form.name.trim(),
    task_groups: form.groups.map((g) => {
      const group: TaskGroupSpec = {
        name: g.name.trim(),
        count: parseInt(g.count, 10) || 1,
        tasks: g.tasks.map((t) => {
          const env: Record<string, string> = {};
          for (const { key, value } of t.envEntries) {
            if (key.trim()) env[key.trim()] = value;
          }
          const task: TaskSpec = {
            name: t.name.trim(),
            image: t.image.trim(),
          };
          if (Object.keys(env).length > 0) task.env = env;
          if (t.ports.length > 0) task.ports = t.ports;
          if (t.cpuMillicores || t.memoryMB) {
            task.resources = {
              cpu: parseInt(t.cpuMillicores, 10) || 0,
              memory: (parseInt(t.memoryMB, 10) || 0) * 1024 * 1024,
            };
          }
          if (t.healthEnabled && t.healthPort) {
            const hc: HealthCheckSpec = {
              type: t.healthType,
              port: parseInt(t.healthPort, 10) || 0,
            };
            if (t.healthType === "http" && t.healthPath) hc.path = t.healthPath;
            if (t.healthType === "exec" && t.healthCommand) {
              hc.command = t.healthCommand.split(/\s+/).filter(Boolean);
            }
            task.health_check = hc;
          }
          return task;
        }),
      };
      if (g.apiAccess) group.api_access = true;
      return group;
    }),
  };
}

// ── Blank defaults ───────────────────────────────────────────────────

export function blankTask(): TaskForm {
  return {
    name: "",
    image: "",
    envEntries: [],
    cpuMillicores: "",
    memoryMB: "",
    ports: [],
    healthEnabled: false,
    healthType: "http",
    healthPort: "",
    healthPath: "/",
    healthCommand: "",
  };
}

export function blankGroup(): GroupForm {
  return { name: "", count: "1", apiAccess: false, tasks: [blankTask()] };
}

export function blankForm(): FormState {
  return { name: "", groups: [blankGroup()] };
}
