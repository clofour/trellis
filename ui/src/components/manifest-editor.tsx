"use client";

import {
  useMemo,
  useRef,
  useState,
  type KeyboardEvent,
} from "react";
import { parseJobManifest } from "@/lib/manifest";

interface ManifestEditorProps {
  value: string;
  onChange: (value: string) => void;
  onDirty?: () => void;
}

type Completion = {
  key: string;
  description: string;
};

const completions: Record<string, Completion[]> = {
  root: [
    { key: "name", description: "Job identifier, unique within its namespace." },
    { key: "namespace", description: "Namespace that owns this job and its allocations." },
    { key: "task_groups", description: "Placement and scaling units for the job." },
  ],
  task_groups: [
    { key: "name", description: "Task-group identifier within the job." },
    { key: "count", description: "Desired number of allocations for this group." },
    { key: "runtime", description: "OCI runtime: runc or runsc when explicitly selected." },
    { key: "labels", description: "Discovery and routing metadata for the group." },
    { key: "api_access", description: "Least-privilege API credential requested by this group." },
    { key: "constraints", description: "Exact node-attribute placement constraints." },
    { key: "restart", description: "Retry policy for failed tasks." },
    { key: "update", description: "Allocation replacement strategy for revisions." },
    { key: "tasks", description: "Containers colocated in every allocation." },
  ],
  tasks: [
    { key: "name", description: "Task identifier within the group." },
    { key: "image", description: "Pullable OCI image reference." },
    { key: "env", description: "Literal environment variables; use secrets for credentials." },
    { key: "networking", description: "Network attachment and host-port reservations." },
    { key: "volumes", description: "Allocation-local or advertised host-volume mounts." },
    { key: "resources", description: "CPU and memory requested by this task." },
    { key: "health_check", description: "HTTP, TCP, or script readiness/health observation." },
    { key: "secrets", description: "Namespace secrets delivered as env vars or files." },
  ],
  resources: [
    { key: "cpu", description: "CPU request in millicores." },
    { key: "memory", description: "Memory request, for example 64MiB or 500MB." },
  ],
  networking: [
    { key: "mode", description: "isolated, host, or namespace networking." },
    { key: "ports", description: "Host ports reserved directly by a host-networked task." },
  ],
  health_check: [
    { key: "type", description: "Health-check implementation: http, tcp, or script." },
    { key: "port", description: "Port checked by HTTP/TCP health checks." },
    { key: "path", description: "HTTP health-check path." },
    { key: "command", description: "Command argv for a script health check." },
    { key: "interval", description: "Check interval, for example 5s." },
    { key: "timeout", description: "Per-check timeout, for example 2s." },
    { key: "threshold", description: "Consecutive failures required before unhealthy." },
  ],
  restart: [
    { key: "max_restarts", description: "Maximum failures allowed inside the restart window." },
    { key: "window", description: "Restart accounting window, for example 5m." },
  ],
  update: [
    { key: "strategy", description: "recreate or rolling replacement." },
    { key: "max_parallel", description: "Maximum not-yet-healthy rolling replacements in flight." },
  ],
  api_access: [
    { key: "scope", description: "Credential scope: namespace or cluster." },
    { key: "access", description: "Credential access: read or write." },
  ],
  constraints: [
    { key: "attribute", description: "Node attribute or label key to match." },
    { key: "value", description: "Exact required value for the attribute." },
  ],
  volumes: [
    { key: "name", description: "Volume name within this task." },
    { key: "path", description: "Absolute container mount path." },
    { key: "host_volume", description: "Optional advertised node volume name." },
    { key: "read_only", description: "Mount the volume read-only when true." },
  ],
  secrets: [
    { key: "name", description: "Stored namespace secret name." },
    { key: "target", description: "Delivery target: env or file." },
    { key: "env", description: "Environment variable name for an env target." },
    { key: "path", description: "Path below /run/trellis-secrets/ for a file target." },
    { key: "mode", description: "File mode: 0400 or 0600 (or decimal equivalent)." },
  ],
  ports: [{ key: "port", description: "Node port reserved and bound directly by the task." }],
};

const containerKeys = new Set(Object.keys(completions).filter((key) => key !== "root"));

function currentContext(source: string, cursor: number): string {
  const before = source.slice(0, cursor);
  const lines = before.split("\n");
  const current = lines.at(-1) ?? "";
  const indent = current.match(/^\s*/)?.[0].length ?? 0;
  const stack: Array<{ indent: number; key: string }> = [];

  for (const line of lines.slice(0, -1)) {
    const match = line.match(/^(\s*)(?:-\s*)?([A-Za-z_][A-Za-z0-9_-]*):\s*(?:#.*)?$/);
    if (!match || !containerKeys.has(match[2])) continue;
    const lineIndent = match[1].length;
    while (stack.length > 0 && stack.at(-1)!.indent >= lineIndent) stack.pop();
    stack.push({ indent: lineIndent, key: match[2] });
  }

  while (stack.length > 0 && stack.at(-1)!.indent >= indent) stack.pop();
  return stack.at(-1)?.key ?? "root";
}

function currentToken(source: string, cursor: number): string {
  const line = source.slice(0, cursor).split("\n").at(-1) ?? "";
  const match = line.match(/(?:^|\s)([A-Za-z_][A-Za-z0-9_-]*)$/);
  return match?.[1] ?? "";
}

export function ManifestEditor({ value, onChange, onDirty }: ManifestEditorProps) {
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const gutterRef = useRef<HTMLDivElement>(null);
  const [cursor, setCursor] = useState(0);
  const [completionOpen, setCompletionOpen] = useState(false);
  const [selectedCompletion, setSelectedCompletion] = useState(0);

  const lineCount = Math.max(1, value.split("\n").length);
  const context = currentContext(value, cursor);
  const token = currentToken(value, cursor).toLowerCase();
  const options = (completions[context] ?? completions.root).filter((item) =>
    item.key.toLowerCase().startsWith(token),
  );

  const yamlError = useMemo(() => {
    try {
      parseJobManifest(value);
      return null;
    } catch (error) {
      return error instanceof Error ? error.message : "Invalid YAML";
    }
  }, [value]);

  const syncCursor = () => {
    setCursor(textareaRef.current?.selectionStart ?? 0);
  };

  const replaceCurrentToken = (key: string) => {
    const textarea = textareaRef.current;
    if (!textarea) return;
    const end = textarea.selectionStart;
    const start = end - currentToken(value, end).length;
    const next = `${value.slice(0, start)}${key}: ${value.slice(end)}`;
    const nextCursor = start + key.length + 2;
    onChange(next);
    onDirty?.();
    setCompletionOpen(false);
    requestAnimationFrame(() => {
      textarea.focus();
      textarea.setSelectionRange(nextCursor, nextCursor);
      setCursor(nextCursor);
    });
  };

  const insertText = (start: number, end: number, text: string, nextCursor: number) => {
    const textarea = textareaRef.current;
    if (!textarea) return;
    onChange(`${value.slice(0, start)}${text}${value.slice(end)}`);
    onDirty?.();
    requestAnimationFrame(() => {
      textarea.focus();
      textarea.setSelectionRange(nextCursor, nextCursor);
      setCursor(nextCursor);
    });
  };

  const handleKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    const textarea = event.currentTarget;

    if ((event.ctrlKey || event.metaKey) && event.key === " ") {
      event.preventDefault();
      setSelectedCompletion(0);
      setCompletionOpen(true);
      return;
    }

    if (completionOpen && options.length > 0) {
      if (event.key === "ArrowDown" || event.key === "ArrowUp") {
        event.preventDefault();
        const delta = event.key === "ArrowDown" ? 1 : -1;
        setSelectedCompletion((index) => (index + delta + options.length) % options.length);
        return;
      }
      if (event.key === "Enter" || event.key === "Tab") {
        event.preventDefault();
        replaceCurrentToken(options[Math.min(selectedCompletion, options.length - 1)].key);
        return;
      }
      if (event.key === "Escape") {
        event.preventDefault();
        setCompletionOpen(false);
        return;
      }
    }

    if (event.key === "Tab") {
      event.preventDefault();
      const start = textarea.selectionStart;
      const end = textarea.selectionEnd;
      insertText(start, end, "  ", start + 2);
      return;
    }

    if (event.key === "Enter" && textarea.selectionStart === textarea.selectionEnd) {
      event.preventDefault();
      const start = textarea.selectionStart;
      const line = value.slice(0, start).split("\n").at(-1) ?? "";
      const indent = line.match(/^\s*/)?.[0] ?? "";
      const extra = /:\s*(?:#.*)?$/.test(line) ? "  " : "";
      const text = `\n${indent}${extra}`;
      insertText(start, start, text, start + text.length);
    }
  };

  return (
    <div className="overflow-hidden rounded-lg border border-border bg-zinc-950 focus-within:ring-2 focus-within:ring-emerald-500/40">
      <div className="flex items-center justify-between border-b border-zinc-800 px-3 py-2 text-[11px] text-zinc-400">
        <div className="flex items-center gap-3">
          <span className="font-medium text-zinc-200">YAML</span>
          <span>Ctrl/⌘ + Space for manifest keys</span>
        </div>
        <span className={yamlError ? "text-amber-400" : "text-emerald-400"}>
          {yamlError ? "YAML needs attention" : "YAML parsed"}
        </span>
      </div>

      <div className="relative flex min-h-[520px] max-h-[70vh]">
        <div
          ref={gutterRef}
          aria-hidden="true"
          className="w-12 shrink-0 overflow-hidden border-r border-zinc-800 bg-zinc-900/70 py-4 text-right font-mono text-xs leading-5 text-zinc-600"
        >
          {Array.from({ length: lineCount }, (_, index) => (
            <div key={index} className="h-5 pr-3">{index + 1}</div>
          ))}
        </div>
        <textarea
          ref={textareaRef}
          id="job-manifest"
          value={value}
          onChange={(event) => {
            onChange(event.target.value);
            onDirty?.();
            setCompletionOpen(false);
          }}
          onClick={syncCursor}
          onKeyUp={syncCursor}
          onSelect={syncCursor}
          onKeyDown={handleKeyDown}
          onScroll={(event) => {
            if (gutterRef.current) gutterRef.current.scrollTop = event.currentTarget.scrollTop;
          }}
          spellCheck={false}
          aria-describedby="manifest-editor-help manifest-editor-diagnostic"
          className="min-h-[520px] flex-1 resize-none overflow-auto bg-zinc-950 p-4 font-mono text-xs leading-5 text-zinc-100 outline-none"
        />

        {completionOpen && (
          <div className="absolute bottom-3 left-14 right-3 z-10 max-h-56 overflow-auto rounded-md border border-zinc-700 bg-zinc-900 p-1 shadow-xl">
            {options.length === 0 ? (
              <p className="px-3 py-2 text-xs text-zinc-400">No manifest keys match here.</p>
            ) : (
              options.map((item, index) => (
                <button
                  key={item.key}
                  type="button"
                  onMouseDown={(event) => event.preventDefault()}
                  onClick={() => replaceCurrentToken(item.key)}
                  className={`flex w-full items-start gap-3 rounded px-3 py-2 text-left ${
                    index === selectedCompletion ? "bg-zinc-800" : "hover:bg-zinc-800/70"
                  }`}
                >
                  <span className="w-28 shrink-0 font-mono text-xs text-emerald-400">{item.key}</span>
                  <span className="text-xs text-zinc-400">{item.description}</span>
                </button>
              ))
            )}
          </div>
        )}
      </div>

      <div className="border-t border-zinc-800 px-3 py-2 text-[11px] text-zinc-400">
        <p id="manifest-editor-help">Tab inserts two spaces; Enter preserves YAML indentation. Completion is editing assistance only; Trellis validation and planning remain authoritative.</p>
        {yamlError && <p id="manifest-editor-diagnostic" className="mt-1 truncate text-amber-400" title={yamlError}>{yamlError}</p>}
      </div>
    </div>
  );
}
