"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { cn } from "@/lib/utils";

const navigation = [
  {
    name: "Overview",
    href: "/",
    icon: (
      <svg width="18" height="18" viewBox="0 0 18 18" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
        <rect x="1.5" y="1.5" width="6" height="6" rx="1" />
        <rect x="10.5" y="1.5" width="6" height="6" rx="1" />
        <rect x="1.5" y="10.5" width="6" height="6" rx="1" />
        <rect x="10.5" y="10.5" width="6" height="6" rx="1" />
      </svg>
    ),
  },
  {
    name: "Nodes",
    href: "/nodes",
    icon: (
      <svg width="18" height="18" viewBox="0 0 18 18" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
        <rect x="3" y="1.5" width="12" height="5" rx="1" />
        <rect x="3" y="11.5" width="12" height="5" rx="1" />
        <path d="M9 6.5v5" />
      </svg>
    ),
  },
  {
    name: "Jobs",
    href: "/jobs",
    icon: (
      <svg width="18" height="18" viewBox="0 0 18 18" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
        <path d="M3 5h12M3 9h12M3 13h8" />
      </svg>
    ),
  },
  {
    name: "Services",
    href: "/services",
    icon: (
      <svg width="18" height="18" viewBox="0 0 18 18" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
        <circle cx="4" cy="9" r="2.5" />
        <circle cx="14" cy="4" r="2.5" />
        <circle cx="14" cy="14" r="2.5" />
        <path d="m6.2 7.8 5.6-2.6M6.2 10.2l5.6 2.6" />
      </svg>
    ),
  },
];

export function Sidebar({ namespace }: { namespace: string }) {
  const pathname = usePathname();

  return (
    <aside className="flex h-full w-56 shrink-0 flex-col border-r border-border bg-card">
      <div className="flex h-14 items-center gap-2.5 border-b border-border px-5">
        <div className="flex h-7 w-7 items-center justify-center rounded-md bg-emerald-600 text-white">
          <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M2 7h10M7 2v10M2 2l10 10M12 2L2 12" />
          </svg>
        </div>
        <span className="text-[15px] font-semibold tracking-tight text-foreground">
          Trellis
        </span>
      </div>

      <nav className="flex flex-1 flex-col gap-0.5 p-3">
        {navigation.map((item) => {
          const isActive =
            item.href === "/"
              ? pathname === "/"
              : pathname.startsWith(item.href);

          return (
            <Link
              key={item.name}
              href={item.href}
              className={cn(
                "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
                isActive
                  ? "bg-accent text-accent-foreground"
                  : "text-muted-foreground hover:bg-accent hover:text-accent-foreground"
              )}
            >
              {item.icon}
              {item.name}
            </Link>
          );
        })}
      </nav>

      <div className="border-t border-border p-4">
        <div className="mb-3">
          <p className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
            Namespace
          </p>
          <p className="mt-1 truncate font-mono text-xs text-foreground" title={namespace}>
            {namespace}
          </p>
        </div>
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          <span className="relative flex h-2 w-2">
            <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-75" />
            <span className="relative inline-flex h-2 w-2 rounded-full bg-emerald-500" />
          </span>
          Orchestrator
        </div>
      </div>
    </aside>
  );
}
