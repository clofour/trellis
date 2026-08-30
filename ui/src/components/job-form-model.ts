import type {
  HealthCheckSpec,
  HealthCheckType,
  JobSpec,
  PortSpec,
  TaskGroupSpec,
  TaskSpec,
} from "@/lib/types";

export interface EnvEntry {
  key: string;
  value: string;
}

export interface TaskForm {
  source: TaskSpec;
  name: string;
  image: string;
  envEntries: EnvEntry[];
  cpuMillicores: string;
  memoryMB: string;
  ports: PortSpec[];
  healthEnabled: boolean;
  healthType: HealthCheckType;
  healthPort: string;
  healthPath: string;
  healthCommand: string;
}

export interface GroupForm {
  source: TaskGroupSpec;
  name: string;
  count: string;
  apiAccess: boolean;
  tasks: TaskForm[];
}

export interface FormState {
  source: JobSpec;
  name: string;
  groups: GroupForm[];
}

function taskToForm(task: TaskSpec): TaskForm {
  return {
    source: task,
    name: task.name,
    image: task.image,
    envEntries: Object.entries(task.env ?? {}).map(([key, value]) => ({ key, value })),
    cpuMillicores: task.resources ? String(task.resources.cpu) : "",
    memoryMB: task.resources ? String(Math.round(task.resources.memory / (1024 * 1024))) : "",
    ports: task.ports ?? [],
    healthEnabled: !!task.health_check,
    healthType: task.health_check?.type ?? "http",
    healthPort: task.health_check ? String(task.health_check.port) : "",
    healthPath: task.health_check?.path ?? "/",
    healthCommand: task.health_check?.command?.join(" ") ?? "",
  };
}

export function specToForm(spec: JobSpec): FormState {
  return {
    source: spec,
    name: spec.name,
    groups: spec.task_groups.map((group) => ({
      source: group,
      name: group.name,
      count: String(group.count),
      apiAccess: group.api_access ?? false,
      tasks: group.tasks.map(taskToForm),
    })),
  };
}

function taskFromForm(task: TaskForm): TaskSpec {
  const env: Record<string, string> = {};
  for (const { key, value } of task.envEntries) {
    if (key.trim()) env[key.trim()] = value;
  }

  const result: TaskSpec = {
    ...task.source,
    name: task.name.trim(),
    image: task.image.trim(),
    env: Object.keys(env).length > 0 ? env : undefined,
    ports: task.ports.length > 0 ? task.ports : undefined,
  };
  if (task.cpuMillicores || task.memoryMB) {
    result.resources = {
      cpu: parseInt(task.cpuMillicores, 10) || 0,
      memory: (parseInt(task.memoryMB, 10) || 0) * 1024 * 1024,
    };
  } else {
    result.resources = undefined;
  }

  if (!task.healthEnabled) {
    result.health_check = undefined;
    return result;
  }

  const health: HealthCheckSpec = {
    ...task.source.health_check,
    type: task.healthType,
    port: parseInt(task.healthPort, 10) || 0,
  };
  health.path = task.healthType === "http" && task.healthPath ? task.healthPath : undefined;
  health.command =
    task.healthType === "script" && task.healthCommand
      ? task.healthCommand.split(/\s+/).filter(Boolean)
      : undefined;
  result.health_check = health;
  return result;
}

export function formToSpec(form: FormState): JobSpec {
  return {
    ...form.source,
    name: form.name.trim(),
    task_groups: form.groups.map((group) => ({
      ...group.source,
      name: group.name.trim(),
      count: parseInt(group.count, 10) || 1,
      api_access: group.apiAccess || undefined,
      tasks: group.tasks.map(taskFromForm),
    })),
  };
}

export function blankTask(): TaskForm {
  const source: TaskSpec = { name: "", image: "" };
  return {
    source,
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
  const task = blankTask();
  const source: TaskGroupSpec = { name: "", count: 1, tasks: [task.source] };
  return { source, name: "", count: "1", apiAccess: false, tasks: [task] };
}

export function blankForm(): FormState {
  const group = blankGroup();
  return { source: { name: "", task_groups: [group.source] }, name: "", groups: [group] };
}

export function validateJobForm(form: FormState): string | null {
  if (!form.name.trim()) return "Job name is required.";
  if (!/^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,62}$/.test(form.name.trim())) {
    return "Job name must start with a letter or digit and contain only letters, digits, hyphens, underscores, or dots.";
  }
  for (const group of form.groups) {
    if (!group.name.trim()) return "All task groups must have a name.";
    const count = parseInt(group.count, 10);
    if (Number.isNaN(count) || count < 1) return "Replica count must be at least 1.";
    for (const task of group.tasks) {
      if (!task.name.trim()) return "All tasks must have a name.";
      if (!task.image.trim()) return "All tasks must have an image.";
      if (task.healthEnabled && !task.healthPort && task.healthType !== "script") {
        return "HTTP and TCP health checks require a port.";
      }
      if (task.healthEnabled && task.healthType === "script" && !task.healthCommand.trim()) {
        return "Script health checks require a command.";
      }
    }
  }
  return null;
}
