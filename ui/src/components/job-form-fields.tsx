import type { HealthCheckType, PortSpec } from "@/lib/types";
import { blankTask, type EnvEntry, type GroupForm, type TaskForm } from "./job-form-model";

export function inputClass(error?: boolean) {
  return `w-full rounded-md border ${error ? "border-red-500" : "border-border"} bg-background px-3 py-1.5 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-emerald-500/40`;
}

function RemoveButton({ onClick, title = "Remove" }: { onClick: () => void; title?: string }) {
  return (
    <button type="button" onClick={onClick} className="flex-shrink-0 text-muted-foreground hover:text-red-500 transition-colors px-1" title={title}>
      <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="2">
        <path d="M2 2l10 10M12 2L2 12" />
      </svg>
    </button>
  );
}

function EnvSection({ entries, onChange }: { entries: EnvEntry[]; onChange: (entries: EnvEntry[]) => void }) {
  const update = (index: number, field: keyof EnvEntry, value: string) =>
    onChange(entries.map((entry, i) => (i === index ? { ...entry, [field]: value } : entry)));
  return (
    <div>
      <label className="block text-xs font-medium text-muted-foreground mb-1.5">Environment Variables</label>
      <div className="space-y-1.5">
        {entries.map((entry, index) => (
          <div key={index} className="flex gap-2">
            <input className={inputClass()} placeholder="KEY" value={entry.key} onChange={(event) => update(index, "key", event.target.value)} style={{ fontFamily: "var(--font-geist-mono)" }} />
            <input className={inputClass()} placeholder="value" value={entry.value} onChange={(event) => update(index, "value", event.target.value)} />
            <RemoveButton onClick={() => onChange(entries.filter((_, i) => i !== index))} />
          </div>
        ))}
      </div>
      <button type="button" onClick={() => onChange([...entries, { key: "", value: "" }])} className="mt-1.5 text-xs text-emerald-600 hover:text-emerald-700 dark:text-emerald-400 dark:hover:text-emerald-300 font-medium">+ Add variable</button>
    </div>
  );
}

function PortsSection({ ports, onChange }: { ports: PortSpec[]; onChange: (ports: PortSpec[]) => void }) {
  const update = (index: number, field: keyof PortSpec, value: string) => {
    const parsed = parseInt(value, 10);
    onChange(ports.map((port, i) => (i === index ? { ...port, [field]: Number.isNaN(parsed) ? 0 : parsed } : port)));
  };
  return (
    <div>
      <label className="block text-xs font-medium text-muted-foreground mb-1.5">Port Mappings</label>
      <div className="space-y-1.5">
        {ports.map((port, index) => (
          <div key={index} className="flex items-center gap-2">
            <div className="flex-1"><input className={inputClass()} placeholder="Host port (0 = dynamic)" value={port.host_port === 0 ? "" : String(port.host_port)} onChange={(event) => update(index, "host_port", event.target.value || "0")} /></div>
            <span className="text-muted-foreground text-sm">→</span>
            <div className="flex-1"><input className={inputClass()} placeholder="Container port" value={port.container_port || ""} onChange={(event) => update(index, "container_port", event.target.value)} /></div>
            <RemoveButton onClick={() => onChange(ports.filter((_, i) => i !== index))} />
          </div>
        ))}
      </div>
      <button type="button" onClick={() => onChange([...ports, { host_port: 0, container_port: 0 }])} className="mt-1.5 text-xs text-emerald-600 hover:text-emerald-700 dark:text-emerald-400 dark:hover:text-emerald-300 font-medium">+ Add port</button>
    </div>
  );
}

function HealthSection({ task, set }: { task: TaskForm; set: <K extends keyof TaskForm>(key: K, value: TaskForm[K]) => void }) {
  return (
    <div>
      <div className="flex items-center gap-2 mb-2">
        <label className="text-xs font-medium text-muted-foreground">Health Check</label>
        <button type="button" role="switch" aria-checked={task.healthEnabled} onClick={() => set("healthEnabled", !task.healthEnabled)} className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors ${task.healthEnabled ? "bg-emerald-500" : "bg-zinc-300 dark:bg-zinc-600"}`}>
          <span className={`inline-block h-3.5 w-3.5 transform rounded-full bg-white shadow transition-transform ${task.healthEnabled ? "translate-x-4" : "translate-x-1"}`} />
        </button>
      </div>
      {task.healthEnabled && (
        <div className="space-y-3 pl-2 border-l-2 border-emerald-500/30">
          <div className="grid grid-cols-3 gap-2">
            <div>
              <label className="block text-xs font-medium text-muted-foreground mb-1">Type</label>
              <select className={inputClass()} value={task.healthType} onChange={(event) => set("healthType", event.target.value as HealthCheckType)}>
                <option value="http">HTTP</option><option value="tcp">TCP</option><option value="script">Script</option>
              </select>
            </div>
            {task.healthType !== "script" && (
              <div className="col-span-2">
                <label className="block text-xs font-medium text-muted-foreground mb-1">Port</label>
                <input className={inputClass()} placeholder="e.g. 80" value={task.healthPort} onChange={(event) => set("healthPort", event.target.value)} />
              </div>
            )}
          </div>
          {task.healthType === "http" && <div><label className="block text-xs font-medium text-muted-foreground mb-1">Path</label><input className={inputClass()} placeholder="/" value={task.healthPath} onChange={(event) => set("healthPath", event.target.value)} /></div>}
          {task.healthType === "script" && <div><label className="block text-xs font-medium text-muted-foreground mb-1">Command</label><input className={inputClass()} placeholder="e.g. curl -f http://localhost/" value={task.healthCommand} onChange={(event) => set("healthCommand", event.target.value)} style={{ fontFamily: "var(--font-geist-mono)" }} /></div>}
        </div>
      )}
    </div>
  );
}

function TaskEditor({ task, index, canRemove, onChange, onRemove }: { task: TaskForm; index: number; canRemove: boolean; onChange: (task: TaskForm) => void; onRemove: () => void }) {
  const set = <K extends keyof TaskForm>(key: K, value: TaskForm[K]) => onChange({ ...task, [key]: value });
  return (
    <div className="rounded-md border border-border bg-background/50 p-4 space-y-4">
      <div className="flex items-center justify-between"><span className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">Task {index + 1}</span>{canRemove && <button type="button" onClick={onRemove} className="text-xs text-muted-foreground hover:text-red-500 transition-colors">Remove task</button>}</div>
      <div className="grid grid-cols-2 gap-3">
        <div><label className="block text-xs font-medium text-muted-foreground mb-1">Name</label><input className={inputClass(!task.name)} placeholder="e.g. web" value={task.name} onChange={(event) => set("name", event.target.value)} /></div>
        <div><label className="block text-xs font-medium text-muted-foreground mb-1">Image</label><input className={inputClass(!task.image)} placeholder="e.g. nginx:latest" value={task.image} onChange={(event) => set("image", event.target.value)} /></div>
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div><label className="block text-xs font-medium text-muted-foreground mb-1">CPU <span className="font-normal">(millicores)</span></label><input className={inputClass()} placeholder="e.g. 500" value={task.cpuMillicores} onChange={(event) => set("cpuMillicores", event.target.value)} /></div>
        <div><label className="block text-xs font-medium text-muted-foreground mb-1">Memory <span className="font-normal">(MB)</span></label><input className={inputClass()} placeholder="e.g. 512" value={task.memoryMB} onChange={(event) => set("memoryMB", event.target.value)} /></div>
      </div>
      <EnvSection entries={task.envEntries} onChange={(entries) => set("envEntries", entries)} />
      <PortsSection ports={task.ports} onChange={(ports) => set("ports", ports)} />
      <HealthSection task={task} set={set} />
    </div>
  );
}

export function GroupEditor({ group, index, canRemove, onChange, onRemove }: { group: GroupForm; index: number; canRemove: boolean; onChange: (group: GroupForm) => void; onRemove: () => void }) {
  const set = <K extends keyof GroupForm>(key: K, value: GroupForm[K]) => onChange({ ...group, [key]: value });
  const updateTask = (taskIndex: number, task: TaskForm) => set("tasks", group.tasks.map((item, i) => (i === taskIndex ? task : item)));
  return (
    <div className="rounded-lg border border-border p-5 space-y-5">
      <div className="flex items-start justify-between"><span className="text-sm font-semibold text-foreground">Task Group {index + 1}</span>{canRemove && <button type="button" onClick={onRemove} className="text-xs text-muted-foreground hover:text-red-500 transition-colors">Remove group</button>}</div>
      <div className="grid grid-cols-2 gap-3">
        <div><label className="block text-xs font-medium text-muted-foreground mb-1">Group Name</label><input className={inputClass(!group.name)} placeholder="e.g. web" value={group.name} onChange={(event) => set("name", event.target.value)} /></div>
        <div><label className="block text-xs font-medium text-muted-foreground mb-1">Replicas</label><input className={inputClass()} type="number" min="1" placeholder="1" value={group.count} onChange={(event) => set("count", event.target.value)} /></div>
      </div>
      <label className="flex items-center gap-2 cursor-pointer"><input type="checkbox" className="h-4 w-4 rounded border-border accent-emerald-600" checked={group.apiAccess} onChange={(event) => set("apiAccess", event.target.checked)} /><span className="text-sm text-foreground">API access</span><span className="text-xs text-muted-foreground">(allow tasks to contact the Trellis API)</span></label>
      <div className="space-y-3">
        <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">Tasks</p>
        {group.tasks.map((task, taskIndex) => <TaskEditor key={taskIndex} task={task} index={taskIndex} canRemove={group.tasks.length > 1} onChange={(next) => updateTask(taskIndex, next)} onRemove={() => set("tasks", group.tasks.filter((_, i) => i !== taskIndex))} />)}
        <button type="button" onClick={() => set("tasks", [...group.tasks, blankTask()])} className="w-full rounded-md border border-dashed border-border py-2 text-xs text-muted-foreground hover:border-emerald-500 hover:text-emerald-600 dark:hover:text-emerald-400 transition-colors">+ Add task</button>
      </div>
    </div>
  );
}
