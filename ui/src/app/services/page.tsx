import { ServicesTable } from "@/components/services-table";

export default function ServicesPage() {
  return (
    <div>
      <div className="mb-6">
        <h1 className="text-xl font-semibold text-foreground">Services</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Healthy service endpoints published by the orchestrator catalog
        </p>
      </div>
      <ServicesTable />
    </div>
  );
}
