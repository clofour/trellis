import type { Job, JobSpec } from "./types";

export interface ManifestChange {
  operation: "add" | "remove" | "change";
  path: string;
  before?: unknown;
  after?: unknown;
}

export interface ManifestPlan {
  action: "create" | "update" | "none";
  title: string;
  changes: ManifestChange[];
}

export function planManifest(current: Job | null, desired: JobSpec): ManifestPlan {
  if (!current?.spec) {
    const desiredAllocations = desired.task_groups.reduce(
      (total, group) => total + group.count,
      0,
    );
    return {
      action: "create",
      title: `Create ${desired.namespace}/${desired.name} (${desiredAllocations} desired allocation${desiredAllocations === 1 ? "" : "s"})`,
      changes: [],
    };
  }

  const changes = diffJobSpecs(current.spec, desired);
  if (changes.length === 0) {
    return {
      action: "none",
      title: `No changes to ${desired.namespace}/${desired.name} (revision ${current.revision})`,
      changes,
    };
  }
  return {
    action: "update",
    title: `Update ${desired.namespace}/${desired.name} from revision ${current.revision}`,
    changes,
  };
}

export function diffJobSpecs(before: JobSpec, after: JobSpec): ManifestChange[] {
  const changes: ManifestChange[] = [];
  walkManifestDiff("", before as unknown, after as unknown, changes);
  return changes;
}

function walkManifestDiff(
  path: string,
  before: unknown,
  after: unknown,
  changes: ManifestChange[],
) {
  if (deepEqual(before, after)) return;

  if (isRecord(before) && isRecord(after)) {
    const keys = Array.from(
      new Set([...Object.keys(before), ...Object.keys(after)]),
    ).sort();
    for (const key of keys) {
      const child = joinManifestPath(path, key);
      const leftExists = Object.prototype.hasOwnProperty.call(before, key);
      const rightExists = Object.prototype.hasOwnProperty.call(after, key);
      if (!leftExists) {
        changes.push({ operation: "add", path: child, after: after[key] });
      } else if (!rightExists) {
        changes.push({ operation: "remove", path: child, before: before[key] });
      } else {
        walkManifestDiff(child, before[key], after[key], changes);
      }
    }
    return;
  }

  if (Array.isArray(before) && Array.isArray(after)) {
    const leftNamed = namedManifestSlice(before);
    const rightNamed = namedManifestSlice(after);
    if (leftNamed && rightNamed) {
      const names = Array.from(
        new Set([...leftNamed.keys(), ...rightNamed.keys()]),
      ).sort();
      for (const name of names) {
        const child = `${path}[${name}]`;
        const leftExists = leftNamed.has(name);
        const rightExists = rightNamed.has(name);
        if (!leftExists) {
          changes.push({ operation: "add", path: child, after: rightNamed.get(name) });
        } else if (!rightExists) {
          changes.push({ operation: "remove", path: child, before: leftNamed.get(name) });
        } else {
          walkManifestDiff(child, leftNamed.get(name), rightNamed.get(name), changes);
        }
      }
      return;
    }

    const limit = Math.max(before.length, after.length);
    for (let index = 0; index < limit; index += 1) {
      const child = `${path}[${index}]`;
      if (index >= before.length) {
        changes.push({ operation: "add", path: child, after: after[index] });
      } else if (index >= after.length) {
        changes.push({ operation: "remove", path: child, before: before[index] });
      } else {
        walkManifestDiff(child, before[index], after[index], changes);
      }
    }
    return;
  }

  changes.push({ operation: "change", path, before, after });
}

function namedManifestSlice(values: unknown[]): Map<string, unknown> | null {
  const result = new Map<string, unknown>();
  for (const value of values) {
    if (!isRecord(value) || typeof value.name !== "string" || !value.name) {
      return null;
    }
    if (result.has(value.name)) return null;
    result.set(value.name, value);
  }
  return result;
}

function joinManifestPath(parent: string, child: string) {
  return parent ? `${parent}.${child}` : child;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function deepEqual(left: unknown, right: unknown): boolean {
  if (Object.is(left, right)) return true;
  if (Array.isArray(left) && Array.isArray(right)) {
    return (
      left.length === right.length &&
      left.every((value, index) => deepEqual(value, right[index]))
    );
  }
  if (isRecord(left) && isRecord(right)) {
    const leftKeys = Object.keys(left);
    const rightKeys = Object.keys(right);
    return (
      leftKeys.length === rightKeys.length &&
      leftKeys.every(
        (key) =>
          Object.prototype.hasOwnProperty.call(right, key) &&
          deepEqual(left[key], right[key]),
      )
    );
  }
  return false;
}

export function formatPlanValue(path: string, value: unknown): string {
  if (value === null || value === undefined) return "null";
  if (typeof value === "number" && isDurationPath(path)) {
    return formatNanoseconds(value);
  }
  if (typeof value === "string") return JSON.stringify(value);
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  return JSON.stringify(value);
}

function isDurationPath(path: string) {
  return (
    path.endsWith(".window") ||
    path.endsWith(".interval") ||
    path.endsWith(".timeout")
  );
}

function formatNanoseconds(value: number): string {
  const units: Array<[number, string]> = [
    [3_600_000_000_000, "h"],
    [60_000_000_000, "m"],
    [1_000_000_000, "s"],
    [1_000_000, "ms"],
    [1_000, "µs"],
    [1, "ns"],
  ];
  if (value === 0) return "0s";
  let remaining = Math.abs(value);
  let result = "";
  for (const [size, suffix] of units) {
    if (remaining < size) continue;
    const amount = Math.floor(remaining / size);
    remaining -= amount * size;
    result += `${amount}${suffix}`;
  }
  return value < 0 ? `-${result}` : result;
}
