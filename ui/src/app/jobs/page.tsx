import { JobsTable } from "@/components/jobs-table";

export default function JobsPage() {
  return (
    <div>
      <div className="mb-6">
        <h1 className="text-xl font-semibold text-foreground">Jobs</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          All registered jobs and their scheduling status
        </p>
      </div>
      <JobsTable />
    </div>
  );
}
