import { DashboardContent } from "@/components/dashboard-content";

export default function OverviewPage() {
  return (
    <div>
      <div className="mb-6">
        <h1 className="text-xl font-semibold text-foreground">Overview</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Cluster health and resource summary
        </p>
      </div>
      <DashboardContent />
    </div>
  );
}
