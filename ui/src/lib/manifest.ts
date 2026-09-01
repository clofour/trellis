import { CORE_SCHEMA, dump, load } from "js-yaml";
import type { DurationValue, JobSpec } from "./types";

export function parseJobManifest(source: string): JobSpec {
  const parsed = load(source, { schema: CORE_SCHEMA });
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    throw new Error("Job manifest must be a YAML mapping.");
  }
  return parsed as JobSpec;
}

export function formatJobManifest(spec: JobSpec): string {
  return dump(humanizeManifestDurations(spec), {
    indent: 2,
    lineWidth: 100,
    noRefs: true,
    sortKeys: false,
  });
}

function humanizeManifestDurations(spec: JobSpec): JobSpec {
  const manifest = JSON.parse(JSON.stringify(spec)) as JobSpec;
  for (const group of manifest.task_groups ?? []) {
    if (group.restart) {
      group.restart.window = humanizeDuration(group.restart.window);
    }
    for (const task of group.tasks ?? []) {
      const check = task.health_check;
      if (!check) continue;
      if (check.interval !== undefined) {
        check.interval = humanizeDuration(check.interval);
      }
      if (check.timeout !== undefined) {
        check.timeout = humanizeDuration(check.timeout);
      }
    }
  }
  return manifest;
}

function humanizeDuration(value: DurationValue): DurationValue {
  if (typeof value !== "number") return value;
  if (!Number.isFinite(value) || value < 0) return value;
  if (value === 0) return "0s";

  const units: Array<[string, number]> = [
    ["h", 3_600_000_000_000],
    ["m", 60_000_000_000],
    ["s", 1_000_000_000],
    ["ms", 1_000_000],
    ["us", 1_000],
    ["ns", 1],
  ];

  let remaining = Math.round(value);
  let result = "";
  for (const [suffix, size] of units) {
    if (remaining < size) continue;
    const amount = Math.floor(remaining / size);
    remaining -= amount * size;
    result += `${amount}${suffix}`;
  }
  return result || `${Math.round(value)}ns`;
}
