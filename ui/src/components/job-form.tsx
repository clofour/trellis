"use client";

import { useCallback, useEffect, useState, type FormEvent } from "react";
import { submitJob } from "@/lib/api";
import type { JobSpec } from "@/lib/types";
import { GroupEditor, inputClass } from "./job-form-fields";
import {
  blankForm,
  blankGroup,
  formToSpec,
  specToForm,
  validateJobForm,
  type FormState,
  type GroupForm,
} from "./job-form-model";

interface JobFormProps {
  open: boolean;
  initialSpec?: JobSpec;
  onClose: () => void;
  onSuccess: () => void;
}

// The outer shell only mounts while open so panel state initializes from props
// exactly once and does not need a synchronization effect.
export function JobForm({ open, initialSpec, onClose, onSuccess }: JobFormProps) {
  if (!open) return null;
  return <JobFormPanel initialSpec={initialSpec} onClose={onClose} onSuccess={onSuccess} />;
}

function JobFormPanel({ initialSpec, onClose, onSuccess }: Omit<JobFormProps, "open">) {
  const [form, setForm] = useState<FormState>(() =>
    initialSpec ? specToForm(initialSpec) : blankForm(),
  );
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

  const updateGroup = (index: number, group: GroupForm) =>
    setForm((current) => ({
      ...current,
      groups: current.groups.map((item, i) => (i === index ? group : item)),
    }));

  const handleSubmit = async (event: FormEvent) => {
    event.preventDefault();
    const validationError = validateJobForm(form);
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

  return (
    <div className="fixed inset-0 z-50 flex" onClick={(event) => { if (event.target === event.currentTarget) handleClose(); }}>
      <div className="absolute inset-0 bg-black/50" onClick={handleClose} />
      <div className="relative ml-auto flex h-full w-full max-w-2xl flex-col bg-card shadow-2xl">
        <div className="flex items-center justify-between border-b border-border px-6 py-4">
          <div>
            <h2 className="text-base font-semibold text-foreground">{isEditing ? `Edit — ${initialSpec.name}` : "New Job"}</h2>
            <p className="mt-0.5 text-xs text-muted-foreground">
              {isEditing ? "Update the job configuration. Changes take effect on the next reconcile." : "Define a new job to schedule on the cluster."}
            </p>
          </div>
          <button type="button" onClick={handleClose} disabled={submitting} className="text-muted-foreground hover:text-foreground transition-colors">
            <svg width="18" height="18" viewBox="0 0 18 18" fill="none" stroke="currentColor" strokeWidth="2"><path d="M3 3l12 12M15 3L3 15" /></svg>
          </button>
        </div>

        <form onSubmit={handleSubmit} className="flex flex-1 flex-col overflow-hidden">
          <div className="flex-1 overflow-y-auto px-6 py-5 space-y-6">
            <div>
              <label className="block text-sm font-medium text-foreground mb-1.5">Job Name <span className="text-red-500">*</span></label>
              <input className={inputClass(!form.name && submitting)} placeholder="e.g. my-app" value={form.name} disabled={isEditing} onChange={(event) => setForm((current) => ({ ...current, name: event.target.value }))} style={isEditing ? { opacity: 0.6, cursor: "not-allowed" } : undefined} />
              {isEditing && <p className="mt-1 text-xs text-muted-foreground">Job name cannot be changed after creation.</p>}
            </div>

            <div className="space-y-4">
              <p className="text-sm font-medium text-foreground">Task Groups</p>
              {form.groups.map((group, index) => (
                <GroupEditor key={index} group={group} index={index} canRemove={form.groups.length > 1} onChange={(next) => updateGroup(index, next)} onRemove={() => setForm((current) => ({ ...current, groups: current.groups.filter((_, i) => i !== index) }))} />
              ))}
              <button type="button" onClick={() => setForm((current) => ({ ...current, groups: [...current.groups, blankGroup()] }))} className="w-full rounded-md border border-dashed border-border py-2.5 text-sm text-muted-foreground hover:border-emerald-500 hover:text-emerald-600 dark:hover:text-emerald-400 transition-colors">+ Add task group</button>
            </div>
          </div>

          <div className="border-t border-border px-6 py-4">
            {error && <p className="mb-3 text-sm text-red-600 dark:text-red-400">{error}</p>}
            <div className="flex justify-end gap-3">
              <button type="button" onClick={handleClose} disabled={submitting} className="rounded-md border border-border bg-background px-4 py-2 text-sm font-medium text-foreground hover:bg-accent transition-colors disabled:opacity-50">Cancel</button>
              <button type="submit" disabled={submitting} className="rounded-md bg-emerald-600 px-4 py-2 text-sm font-medium text-white hover:bg-emerald-700 transition-colors disabled:opacity-50 min-w-[100px]">{submitting ? "Saving…" : isEditing ? "Update Job" : "Create Job"}</button>
            </div>
          </div>
        </form>
      </div>
    </div>
  );
}
