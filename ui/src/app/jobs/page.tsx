"use client";

import { useState } from "react";
import { JobsTable } from "@/components/jobs-table";
import { JobForm } from "@/components/job-form";
import { useConfig } from "@/components/config-provider";

export default function JobsPage() {
  const { allowWrites } = useConfig();
  const [formOpen, setFormOpen] = useState(false);
  const [refreshKey, setRefreshKey] = useState(0);

  return (
    <div>
      <div className="mb-6 flex items-start justify-between">
        <div>
          <h1 className="text-xl font-semibold text-foreground">Jobs</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            All registered jobs and their scheduling status
          </p>
        </div>
        {allowWrites && (
          <button
            onClick={() => setFormOpen(true)}
            className="flex items-center gap-2 rounded-md bg-emerald-600 px-4 py-2 text-sm font-medium text-white hover:bg-emerald-700 transition-colors"
          >
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M7 1v12M1 7h12" />
            </svg>
            New Job
          </button>
        )}
      </div>
      <JobsTable key={refreshKey} />
      <JobForm
        open={formOpen}
        onClose={() => setFormOpen(false)}
        onSuccess={() => setRefreshKey((k) => k + 1)}
      />
    </div>
  );
}
