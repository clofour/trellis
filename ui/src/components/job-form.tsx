"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import type { JobSpec } from "@/lib/types";
import { submitJob } from "@/lib/api";
import { useConfig } from "./config-provider";

interface JobFormProps {
  open: boolean;
  initialSpec?: JobSpec;
  onClose: () => void;
  onSuccess: () => void;
}

function defaultSpec(namespace: string): JobSpec {
  return {
    name: "",
    namespace,
    task_groups: [
      {
        name: "web",
        count: 1,
        tasks: [
          {
            name: "web",
            image: "nginx:latest",
            resources: { cpu: 250, memory: 268435456 },
          },
        ],
      },
    ],
  };
}

function pretty(spec: JobSpec): string {
  return JSON.stringify(spec, null, 2);
}

export function JobForm({
  open,
  initialSpec,
  onClose,
  onSuccess,
}: JobFormProps) {
  if (!open) return null;
  return (
    <JobFormPanel
      initialSpec={initialSpec}
      onClose={onClose}
      onSuccess={onSuccess}
    />
  );
}

function JobFormPanel({
  initialSpec,
  onClose,
  onSuccess,
}: {
  initialSpec?: JobSpec;
  onClose: () => void;
  onSuccess: () => void;
}) {
  const { namespace } = useConfig();
  const initial = useMemo(() => {
    const spec = initialSpec
      ? JSON.parse(JSON.stringify(initialSpec)) as JobSpec
      : defaultSpec(namespace);
    spec.namespace = namespace;
    return spec;
  }, [initialSpec, namespace]);
  const [source, setSource] = useState(() => pretty(initial));
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const isEditing = !!initialSpec;

  const handleClose = useCallback(() => {
    if (!submitting) onClose();
  }, [submitting, onClose]);

  useEffect(() => {
    const handleKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") handleClose();
    };
    document.addEventListener("keydown", handleKey);
    return () => document.removeEventListener("keydown", handleKey);
  }, [handleClose]);

  const parse = (): JobSpec => {
    let parsed: unknown;
    try {
      parsed = JSON.parse(source);
    } catch (err) {
      throw new Error(
        err instanceof Error ? `Invalid JSON: ${err.message}` : "Invalid JSON",
      );
    }
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      throw new Error("Job spec must be a JSON object.");
    }
    const spec = parsed as JobSpec;
    if (!spec.name || typeof spec.name !== "string") {
      throw new Error("Job name is required.");
    }
    if (!/^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,62}$/.test(spec.name)) {
      throw new Error(
        "Job name must start with a letter or digit and contain only letters, digits, hyphens, underscores, or dots.",
      );
    }
    if (!Array.isArray(spec.task_groups) || spec.task_groups.length === 0) {
      throw new Error("At least one task group is required.");
    }
    if (isEditing && spec.name !== initialSpec!.name) {
      throw new Error("Job name cannot be changed after creation.");
    }
    // The dashboard is scoped to one namespace. The server proxy enforces this
    // again so a modified browser request cannot escape the configured scope.
    spec.namespace = namespace;
    return spec;
  };

  const formatSource = () => {
    try {
      const spec = parse();
      setSource(pretty(spec));
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Invalid job spec");
    }
  };

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      const spec = parse();
      await submitJob(spec);
      onSuccess();
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unexpected error");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex" role="dialog" aria-modal="true">
      <button
        type="button"
        aria-label="Close job editor"
        className="absolute inset-0 bg-black/50"
        onClick={handleClose}
      />
      <div className="relative ml-auto flex h-full w-full max-w-3xl flex-col bg-card shadow-2xl">
        <div className="flex items-start justify-between border-b border-border px-6 py-4">
          <div>
            <h2 className="text-base font-semibold text-foreground">
              {isEditing ? `Edit — ${initialSpec!.name}` : "New Job"}
            </h2>
            <p className="mt-1 text-xs text-muted-foreground">
              Full Trellis job spec. This JSON representation is lossless and supports every manifest field.
            </p>
          </div>
          <button
            type="button"
            onClick={handleClose}
            disabled={submitting}
            className="rounded-md p-1 text-muted-foreground hover:bg-accent hover:text-foreground disabled:opacity-50"
          >
            <span className="sr-only">Close</span>
            <svg width="18" height="18" viewBox="0 0 18 18" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M3 3l12 12M15 3L3 15" />
            </svg>
          </button>
        </div>

        <form onSubmit={handleSubmit} className="flex flex-1 flex-col overflow-hidden">
          <div className="flex-1 overflow-y-auto px-6 py-5">
            <div className="mb-4 rounded-lg border border-border bg-background/60 p-4 text-xs text-muted-foreground">
              <p>
                Namespace is fixed to <span className="font-mono text-foreground">{namespace || "(unscoped)"}</span> by the dashboard.
              </p>
              <p className="mt-2">
                Supported fields include WireGuard networking, task-group runtime and host networking, labels, restart policy, placement constraints, rolling/recreate updates, resources, ports, volumes, secrets, API access, and configurable health checks.
              </p>
            </div>

            <div className="mb-2 flex items-center justify-between">
              <label htmlFor="job-spec" className="text-sm font-medium text-foreground">
                Job spec
              </label>
              <button
                type="button"
                onClick={formatSource}
                className="rounded-md border border-border bg-background px-2.5 py-1.5 text-xs font-medium text-foreground hover:bg-accent"
              >
                Format JSON
              </button>
            </div>
            <textarea
              id="job-spec"
              value={source}
              onChange={(event) => setSource(event.target.value)}
              spellCheck={false}
              className="min-h-[520px] w-full resize-y rounded-lg border border-border bg-zinc-950 p-4 font-mono text-xs leading-5 text-zinc-100 outline-none focus:ring-2 focus:ring-emerald-500/40"
            />
          </div>

          <div className="border-t border-border px-6 py-4">
            {error && (
              <p className="mb-3 rounded-md border border-red-500/20 bg-red-500/5 px-3 py-2 text-sm text-red-600 dark:text-red-400">
                {error}
              </p>
            )}
            <div className="flex justify-end gap-3">
              <button
                type="button"
                onClick={handleClose}
                disabled={submitting}
                className="rounded-md border border-border bg-background px-4 py-2 text-sm font-medium text-foreground hover:bg-accent disabled:opacity-50"
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={submitting}
                className="min-w-[100px] rounded-md bg-emerald-600 px-4 py-2 text-sm font-medium text-white hover:bg-emerald-700 disabled:opacity-50"
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
