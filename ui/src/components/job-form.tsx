"use client";

import { useState, useEffect, useCallback } from "react";
import type { JobSpec, TaskGroupSpec, TaskSpec, PortSpec, HealthCheckSpec } from "@/lib/types";
import { submitJob } from "@/lib/api";

// ── Internal form state types ────────────────────────────────────────

interface EnvEntry {
  key: string;
  value: string;
}

interface TaskForm {
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

interface GroupForm {
  name: string;
  count: string;
  apiAccess: boolean;
  tasks: TaskForm[];
}

interface FormState {
  name: string;
  groups: GroupForm[];
}

// ── Conversions ──────────────────────────────────────────────────────

function specToForm(spec: JobSpec): FormState {
  return {
    name: spec.name,
    groups: spec.task_groups.map((g) => ({
      name: g.name,
      count: String(g.count),
      apiAccess: g.api_access ?? false,
      tasks: g.tasks.map((t) => ({
        name: t.name,
        image: t.image,
        envEntries: Object.entries(t.env ?? {}).map(([key, value]) => ({ key, value })),
        cpuMillicores: t.resources ? String(t.resources.cpu) : "",
        memoryMB: t.resources ? String(Math.round(t.resources.memory / (1024 * 1024))) : "",
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

function formToSpec(form: FormState): JobSpec {
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

function blankTask(): TaskForm {
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

function blankGroup(): GroupForm {
  return { name: "", count: "1", apiAccess: false, tasks: [blankTask()] };
}

function blankForm(): FormState {
  return { name: "", groups: [blankGroup()] };
}

// ── Helpers ──────────────────────────────────────────────────────────

function inputClass(error?: boolean) {
  return `w-full rounded-md border ${error ? "border-red-500" : "border-border"} bg-background px-3 py-1.5 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-emerald-500/40`;
}

// ── Sub-components ───────────────────────────────────────────────────

function EnvSection({
  entries,
  onChange,
}: {
  entries: EnvEntry[];
  onChange: (entries: EnvEntry[]) => void;
}) {
  const update = (i: number, field: keyof EnvEntry, val: string) => {
    const next = entries.map((e, idx) => (idx === i ? { ...e, [field]: val } : e));
    onChange(next);
  };
  const remove = (i: number) => onChange(entries.filter((_, idx) => idx !== i));
  const add = () => onChange([...entries, { key: "", value: "" }]);

  return (
    <div>
      <label className="block text-xs font-medium text-muted-foreground mb-1.5">
        Environment Variables
      </label>
      <div className="space-y-1.5">
        {entries.map((e, i) => (
          <div key={i} className="flex gap-2">
            <input
              className={inputClass()}
              placeholder="KEY"
              value={e.key}
              onChange={(ev) => update(i, "key", ev.target.value)}
              style={{ fontFamily: "var(--font-geist-mono)" }}
            />
            <input
              className={inputClass()}
              placeholder="value"
              value={e.value}
              onChange={(ev) => update(i, "value", ev.target.value)}
            />
            <button
              type="button"
              onClick={() => remove(i)}
              className="flex-shrink-0 text-muted-foreground hover:text-red-500 transition-colors px-1"
              title="Remove"
            >
              <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M2 2l10 10M12 2L2 12" />
              </svg>
            </button>
          </div>
        ))}
      </div>
      <button
        type="button"
        onClick={add}
        className="mt-1.5 text-xs text-emerald-600 hover:text-emerald-700 dark:text-emerald-400 dark:hover:text-emerald-300 font-medium"
      >
        + Add variable
      </button>
    </div>
  );
}

function PortsSection({
  ports,
  onChange,
}: {
  ports: PortSpec[];
  onChange: (ports: PortSpec[]) => void;
}) {
  const update = (i: number, field: keyof PortSpec, val: string) => {
    const n = parseInt(val, 10);
    const next = ports.map((p, idx) =>
      idx === i ? { ...p, [field]: isNaN(n) ? 0 : n } : p
    );
    onChange(next);
  };
  const remove = (i: number) => onChange(ports.filter((_, idx) => idx !== i));
  const add = () => onChange([...ports, { host_port: 0, container_port: 0 }]);

  return (
    <div>
      <label className="block text-xs font-medium text-muted-foreground mb-1.5">
        Port Mappings
      </label>
      <div className="space-y-1.5">
        {ports.map((p, i) => (
          <div key={i} className="flex items-center gap-2">
            <div className="flex-1">
              <input
                className={inputClass()}
                placeholder="Host port (0 = dynamic)"
                value={p.host_port === 0 ? "" : String(p.host_port)}
                onChange={(ev) => update(i, "host_port", ev.target.value || "0")}
              />
            </div>
            <span className="text-muted-foreground text-sm">→</span>
            <div className="flex-1">
              <input
                className={inputClass()}
                placeholder="Container port"
                value={p.container_port || ""}
                onChange={(ev) => update(i, "container_port", ev.target.value)}
              />
            </div>
            <button
              type="button"
              onClick={() => remove(i)}
              className="flex-shrink-0 text-muted-foreground hover:text-red-500 transition-colors px-1"
            >
              <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M2 2l10 10M12 2L2 12" />
              </svg>
            </button>
          </div>
        ))}
      </div>
      <button
        type="button"
        onClick={add}
        className="mt-1.5 text-xs text-emerald-600 hover:text-emerald-700 dark:text-emerald-400 dark:hover:text-emerald-300 font-medium"
      >
        + Add port
      </button>
    </div>
  );
}

function TaskEditor({
  task,
  index,
  canRemove,
  onChange,
  onRemove,
}: {
  task: TaskForm;
  index: number;
  canRemove: boolean;
  onChange: (t: TaskForm) => void;
  onRemove: () => void;
}) {
  const set = <K extends keyof TaskForm>(k: K, v: TaskForm[K]) =>
    onChange({ ...task, [k]: v });

  return (
    <div className="rounded-md border border-border bg-background/50 p-4 space-y-4">
      <div className="flex items-center justify-between">
        <span className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
          Task {index + 1}
        </span>
        {canRemove && (
          <button
            type="button"
            onClick={onRemove}
            className="text-xs text-muted-foreground hover:text-red-500 transition-colors"
          >
            Remove task
          </button>
        )}
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div>
          <label className="block text-xs font-medium text-muted-foreground mb-1">Name</label>
          <input
            className={inputClass(!task.name)}
            placeholder="e.g. web"
            value={task.name}
            onChange={(e) => set("name", e.target.value)}
          />
        </div>
        <div>
          <label className="block text-xs font-medium text-muted-foreground mb-1">Image</label>
          <input
            className={inputClass(!task.image)}
            placeholder="e.g. nginx:latest"
            value={task.image}
            onChange={(e) => set("image", e.target.value)}
          />
        </div>
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div>
          <label className="block text-xs font-medium text-muted-foreground mb-1">
            CPU <span className="font-normal">(millicores)</span>
          </label>
          <input
            className={inputClass()}
            placeholder="e.g. 500"
            value={task.cpuMillicores}
            onChange={(e) => set("cpuMillicores", e.target.value)}
          />
        </div>
        <div>
          <label className="block text-xs font-medium text-muted-foreground mb-1">
            Memory <span className="font-normal">(MB)</span>
          </label>
          <input
            className={inputClass()}
            placeholder="e.g. 512"
            value={task.memoryMB}
            onChange={(e) => set("memoryMB", e.target.value)}
          />
        </div>
      </div>

      <EnvSection
        entries={task.envEntries}
        onChange={(entries) => set("envEntries", entries)}
      />

      <PortsSection
        ports={task.ports}
        onChange={(ports) => set("ports", ports)}
      />

      <div>
        <div className="flex items-center gap-2 mb-2">
          <label className="text-xs font-medium text-muted-foreground">Health Check</label>
          <button
            type="button"
            role="switch"
            aria-checked={task.healthEnabled}
            onClick={() => set("healthEnabled", !task.healthEnabled)}
            className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors ${
              task.healthEnabled ? "bg-emerald-500" : "bg-zinc-300 dark:bg-zinc-600"
            }`}
          >
            <span
              className={`inline-block h-3.5 w-3.5 transform rounded-full bg-white shadow transition-transform ${
                task.healthEnabled ? "translate-x-4" : "translate-x-1"
              }`}
            />
          </button>
        </div>

        {task.healthEnabled && (
          <div className="space-y-3 pl-2 border-l-2 border-emerald-500/30">
            <div className="grid grid-cols-3 gap-2">
              <div>
                <label className="block text-xs font-medium text-muted-foreground mb-1">Type</label>
                <select
                  className={inputClass()}
                  value={task.healthType}
                  onChange={(e) => set("healthType", e.target.value as "http" | "tcp" | "exec")}
                >
                  <option value="http">HTTP</option>
                  <option value="tcp">TCP</option>
                  <option value="exec">Exec</option>
                </select>
              </div>
              <div className="col-span-2">
                <label className="block text-xs font-medium text-muted-foreground mb-1">Port</label>
                <input
                  className={inputClass()}
                  placeholder="e.g. 80"
                  value={task.healthPort}
                  onChange={(e) => set("healthPort", e.target.value)}
                />
              </div>
            </div>
            {task.healthType === "http" && (
              <div>
                <label className="block text-xs font-medium text-muted-foreground mb-1">Path</label>
                <input
                  className={inputClass()}
                  placeholder="/"
                  value={task.healthPath}
                  onChange={(e) => set("healthPath", e.target.value)}
                />
              </div>
            )}
            {task.healthType === "exec" && (
              <div>
                <label className="block text-xs font-medium text-muted-foreground mb-1">Command</label>
                <input
                  className={inputClass()}
                  placeholder="e.g. curl -f http://localhost/"
                  value={task.healthCommand}
                  onChange={(e) => set("healthCommand", e.target.value)}
                  style={{ fontFamily: "var(--font-geist-mono)" }}
                />
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

function GroupEditor({
  group,
  index,
  canRemove,
  onChange,
  onRemove,
}: {
  group: GroupForm;
  index: number;
  canRemove: boolean;
  onChange: (g: GroupForm) => void;
  onRemove: () => void;
}) {
  const set = <K extends keyof GroupForm>(k: K, v: GroupForm[K]) =>
    onChange({ ...group, [k]: v });

  const addTask = () => set("tasks", [...group.tasks, blankTask()]);
  const updateTask = (i: number, t: TaskForm) =>
    set("tasks", group.tasks.map((x, idx) => (idx === i ? t : x)));
  const removeTask = (i: number) =>
    set("tasks", group.tasks.filter((_, idx) => idx !== i));

  return (
    <div className="rounded-lg border border-border p-5 space-y-5">
      <div className="flex items-start justify-between">
        <span className="text-sm font-semibold text-foreground">
          Task Group {index + 1}
        </span>
        {canRemove && (
          <button
            type="button"
            onClick={onRemove}
            className="text-xs text-muted-foreground hover:text-red-500 transition-colors"
          >
            Remove group
          </button>
        )}
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div>
          <label className="block text-xs font-medium text-muted-foreground mb-1">Group Name</label>
          <input
            className={inputClass(!group.name)}
            placeholder="e.g. web"
            value={group.name}
            onChange={(e) => set("name", e.target.value)}
          />
        </div>
        <div>
          <label className="block text-xs font-medium text-muted-foreground mb-1">Replicas</label>
          <input
            className={inputClass()}
            type="number"
            min="1"
            placeholder="1"
            value={group.count}
            onChange={(e) => set("count", e.target.value)}
          />
        </div>
      </div>

      <label className="flex items-center gap-2 cursor-pointer">
        <input
          type="checkbox"
          className="h-4 w-4 rounded border-border accent-emerald-600"
          checked={group.apiAccess}
          onChange={(e) => set("apiAccess", e.target.checked)}
        />
        <span className="text-sm text-foreground">API access</span>
        <span className="text-xs text-muted-foreground">(allow tasks to contact the Trellis API)</span>
      </label>

      <div className="space-y-3">
        <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">Tasks</p>
        {group.tasks.map((task, i) => (
          <TaskEditor
            key={i}
            task={task}
            index={i}
            canRemove={group.tasks.length > 1}
            onChange={(t) => updateTask(i, t)}
            onRemove={() => removeTask(i)}
          />
        ))}
        <button
          type="button"
          onClick={addTask}
          className="w-full rounded-md border border-dashed border-border py-2 text-xs text-muted-foreground hover:border-emerald-500 hover:text-emerald-600 dark:hover:text-emerald-400 transition-colors"
        >
          + Add task
        </button>
      </div>
    </div>
  );
}

// ── Validation ───────────────────────────────────────────────────────

function validate(form: FormState): string | null {
  if (!form.name.trim()) return "Job name is required.";
  if (!/^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,62}$/.test(form.name.trim()))
    return "Job name must start with a letter or digit and contain only letters, digits, hyphens, underscores, or dots.";
  for (const g of form.groups) {
    if (!g.name.trim()) return "All task groups must have a name.";
    const count = parseInt(g.count, 10);
    if (isNaN(count) || count < 1) return "Replica count must be at least 1.";
    for (const t of g.tasks) {
      if (!t.name.trim()) return "All tasks must have a name.";
      if (!t.image.trim()) return "All tasks must have an image.";
    }
  }
  return null;
}

// ── Main component ───────────────────────────────────────────────────

interface JobFormProps {
  open: boolean;
  initialSpec?: JobSpec;
  onClose: () => void;
  onSuccess: () => void;
}

export function JobForm({ open, initialSpec, onClose, onSuccess }: JobFormProps) {
  const [form, setForm] = useState<FormState>(blankForm);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const isEditing = !!initialSpec;

  useEffect(() => {
    if (open) {
      setForm(initialSpec ? specToForm(initialSpec) : blankForm());
      setError(null);
    }
  }, [open, initialSpec]);

  const handleClose = useCallback(() => {
    if (!submitting) onClose();
  }, [submitting, onClose]);

  useEffect(() => {
    if (!open) return;
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") handleClose();
    };
    document.addEventListener("keydown", handleKey);
    return () => document.removeEventListener("keydown", handleKey);
  }, [open, handleClose]);

  const addGroup = () =>
    setForm((f) => ({ ...f, groups: [...f.groups, blankGroup()] }));
  const updateGroup = (i: number, g: GroupForm) =>
    setForm((f) => ({ ...f, groups: f.groups.map((x, idx) => (idx === i ? g : x)) }));
  const removeGroup = (i: number) =>
    setForm((f) => ({ ...f, groups: f.groups.filter((_, idx) => idx !== i) }));

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const validationError = validate(form);
    if (validationError) {
      setError(validationError);
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      await submitJob(formToSpec(form));
      onSuccess();
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unexpected error");
    } finally {
      setSubmitting(false);
    }
  };

  if (!open) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex"
      onClick={(e) => {
        if (e.target === e.currentTarget) handleClose();
      }}
    >
      {/* Overlay */}
      <div className="absolute inset-0 bg-black/50" onClick={handleClose} />

      {/* Panel */}
      <div className="relative ml-auto flex h-full w-full max-w-2xl flex-col bg-card shadow-2xl">
        {/* Header */}
        <div className="flex items-center justify-between border-b border-border px-6 py-4">
          <div>
            <h2 className="text-base font-semibold text-foreground">
              {isEditing ? `Edit — ${initialSpec!.name}` : "New Job"}
            </h2>
            <p className="mt-0.5 text-xs text-muted-foreground">
              {isEditing
                ? "Update the job configuration. Changes take effect on the next reconcile."
                : "Define a new job to schedule on the cluster."}
            </p>
          </div>
          <button
            type="button"
            onClick={handleClose}
            disabled={submitting}
            className="text-muted-foreground hover:text-foreground transition-colors"
          >
            <svg width="18" height="18" viewBox="0 0 18 18" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M3 3l12 12M15 3L3 15" />
            </svg>
          </button>
        </div>

        {/* Scrollable body */}
        <form onSubmit={handleSubmit} className="flex flex-1 flex-col overflow-hidden">
          <div className="flex-1 overflow-y-auto px-6 py-5 space-y-6">
            {/* Job name */}
            <div>
              <label className="block text-sm font-medium text-foreground mb-1.5">
                Job Name <span className="text-red-500">*</span>
              </label>
              <input
                className={inputClass(!form.name && submitting)}
                placeholder="e.g. my-app"
                value={form.name}
                disabled={isEditing}
                onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
                style={isEditing ? { opacity: 0.6, cursor: "not-allowed" } : undefined}
              />
              {isEditing && (
                <p className="mt-1 text-xs text-muted-foreground">
                  Job name cannot be changed after creation.
                </p>
              )}
            </div>

            {/* Task groups */}
            <div className="space-y-4">
              <p className="text-sm font-medium text-foreground">Task Groups</p>
              {form.groups.map((group, i) => (
                <GroupEditor
                  key={i}
                  group={group}
                  index={i}
                  canRemove={form.groups.length > 1}
                  onChange={(g) => updateGroup(i, g)}
                  onRemove={() => removeGroup(i)}
                />
              ))}
              <button
                type="button"
                onClick={addGroup}
                className="w-full rounded-md border border-dashed border-border py-2.5 text-sm text-muted-foreground hover:border-emerald-500 hover:text-emerald-600 dark:hover:text-emerald-400 transition-colors"
              >
                + Add task group
              </button>
            </div>
          </div>

          {/* Footer */}
          <div className="border-t border-border px-6 py-4">
            {error && (
              <p className="mb-3 text-sm text-red-600 dark:text-red-400">{error}</p>
            )}
            <div className="flex justify-end gap-3">
              <button
                type="button"
                onClick={handleClose}
                disabled={submitting}
                className="rounded-md border border-border bg-background px-4 py-2 text-sm font-medium text-foreground hover:bg-accent transition-colors disabled:opacity-50"
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={submitting}
                className="rounded-md bg-emerald-600 px-4 py-2 text-sm font-medium text-white hover:bg-emerald-700 transition-colors disabled:opacity-50 min-w-[100px]"
              >
                {submitting ? "Saving…" : isEditing ? "Update Job" : "Create Job"}
              </button>
            </div>
          </div>
        </form>
      </div>
    </div>
  );
}
