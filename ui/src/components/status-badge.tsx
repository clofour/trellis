import { cn } from "@/lib/utils";

type Variant = "healthy" | "unhealthy" | "pending" | "draining" | "running";

const styles: Record<Variant, string> = {
  healthy:
    "bg-emerald-50 text-emerald-700 ring-emerald-600/20 dark:bg-emerald-500/10 dark:text-emerald-400 dark:ring-emerald-500/20",
  running:
    "bg-emerald-50 text-emerald-700 ring-emerald-600/20 dark:bg-emerald-500/10 dark:text-emerald-400 dark:ring-emerald-500/20",
  unhealthy:
    "bg-red-50 text-red-700 ring-red-600/20 dark:bg-red-500/10 dark:text-red-400 dark:ring-red-500/20",
  pending:
    "bg-amber-50 text-amber-700 ring-amber-600/20 dark:bg-amber-500/10 dark:text-amber-400 dark:ring-amber-500/20",
  draining:
    "bg-blue-50 text-blue-700 ring-blue-600/20 dark:bg-blue-500/10 dark:text-blue-400 dark:ring-blue-500/20",
};

const dots: Record<Variant, string> = {
  healthy: "bg-emerald-500",
  running: "bg-emerald-500",
  unhealthy: "bg-red-500",
  pending: "bg-amber-500",
  draining: "bg-blue-500",
};

export function StatusBadge({
  status,
  className,
}: {
  status: string;
  className?: string;
}) {
  const variant = (status in styles ? status : "pending") as Variant;

  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium ring-1 ring-inset",
        styles[variant],
        className
      )}
    >
      <span className={cn("h-1.5 w-1.5 rounded-full", dots[variant])} />
      {status}
    </span>
  );
}
