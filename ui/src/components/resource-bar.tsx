import { cn } from "@/lib/utils";

export function ResourceBar({
  label,
  used,
  total,
  format = "raw",
}: {
  label: string;
  used: number;
  total: number;
  format?: "raw" | "cpu" | "memory";
}) {
  const pct = total > 0 ? Math.min((used / total) * 100, 100) : 0;

  function formatValue(val: number) {
    if (format === "cpu") {
      return val >= 1000
        ? `${(val / 1000).toFixed(1)} cores`
        : `${val}m`;
    }
    if (format === "memory") {
      if (val >= 1073741824)
        return `${(val / 1073741824).toFixed(1)} GiB`;
      if (val >= 1048576)
        return `${(val / 1048576).toFixed(0)} MiB`;
      return `${val} B`;
    }
    return String(val);
  }

  return (
    <div className="space-y-1.5">
      <div className="flex items-center justify-between text-sm">
        <span className="font-medium text-card-foreground">{label}</span>
        <span className="tabular-nums text-muted-foreground">
          {formatValue(used)} / {formatValue(total)}
          <span className="ml-1.5 text-xs">({pct.toFixed(0)}%)</span>
        </span>
      </div>
      <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
        <div
          className={cn(
            "h-full rounded-full transition-all duration-500",
            pct > 85
              ? "bg-red-500"
              : pct > 65
                ? "bg-amber-500"
                : "bg-emerald-500"
          )}
          style={{ width: `${pct}%` }}
        />
      </div>
    </div>
  );
}
