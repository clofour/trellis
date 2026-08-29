import { NodesTable } from "@/components/nodes-table";

export default function NodesPage() {
  return (
    <div>
      <div className="mb-6">
        <h1 className="text-xl font-semibold text-foreground">Nodes</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          All registered cluster nodes and their current status
        </p>
      </div>
      <NodesTable />
    </div>
  );
}
