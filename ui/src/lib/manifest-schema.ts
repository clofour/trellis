export interface ManifestSchema {
  $ref?: string;
  $defs?: Record<string, ManifestSchema>;
  type?: string;
  description?: string;
  properties?: Record<string, ManifestSchema>;
  required?: string[];
  additionalProperties?: boolean | ManifestSchema;
  items?: ManifestSchema;
  enum?: unknown[];
  const?: unknown;
  pattern?: string;
  minimum?: number;
  maximum?: number;
  minLength?: number;
  maxLength?: number;
  minItems?: number;
  maxItems?: number;
  oneOf?: ManifestSchema[];
  allOf?: ManifestSchema[];
  if?: ManifestSchema;
  then?: ManifestSchema;
  not?: ManifestSchema;
}

export interface ManifestCompletion {
  key: string;
  description: string;
}

export interface ManifestValueCompletion {
  label: string;
  value: string;
  description: string;
}

let schemaPromise: Promise<ManifestSchema> | null = null;

export function getManifestSchema(): Promise<ManifestSchema> {
  if (!schemaPromise) {
    schemaPromise = fetch("/trellis-job.schema.json", { cache: "force-cache" }).then(
      async (response) => {
        if (!response.ok) {
          throw new Error(`manifest schema unavailable: ${response.status}`);
        }
        return (await response.json()) as ManifestSchema;
      },
    );
  }
  return schemaPromise;
}

export function manifestContainerKeys(root: ManifestSchema): Set<string> {
  const result = new Set<string>();
  const seen = new Set<ManifestSchema>();

  const visit = (input: ManifestSchema) => {
    const schema = resolveSchema(root, input);
    if (seen.has(schema)) return;
    seen.add(schema);
    for (const [key, property] of Object.entries(schema.properties ?? {})) {
      const child = objectValueSchema(root, property);
      if (!child) continue;
      result.add(key);
      visit(child);
    }
  };

  visit(root);
  return result;
}

export function manifestCompletions(
  root: ManifestSchema,
  context: string,
): ManifestCompletion[] {
  const schema = contextSchema(root, context);
  if (!schema) return [];
  return Object.entries(schema.properties ?? {})
    .map(([key, property]) => ({
      key,
      description:
        resolveSchema(root, property).description ??
        property.description ??
        "Trellis manifest field.",
    }))
    .sort((a, b) => a.key.localeCompare(b.key));
}

export function manifestValueCompletions(
  root: ManifestSchema,
  context: string,
  key: string,
): ManifestValueCompletion[] {
  const schema = contextSchema(root, context);
  const property = schema?.properties?.[key];
  if (!property) return [];

  const description =
    resolveSchema(root, property).description ??
    property.description ??
    "Allowed value from the Trellis manifest schema.";
  const values = scalarCompletionValues(root, property);
  const seen = new Set<string>();
  const result: ManifestValueCompletion[] = [];
  for (const value of values) {
    const insert = yamlScalar(value);
    if (seen.has(insert)) continue;
    seen.add(insert);
    result.push({
      label: String(value),
      value: insert,
      description,
    });
  }
  return result;
}

export function validateManifestShape(
  root: ManifestSchema,
  value: unknown,
): string | null {
  return validateNode(root, root, value, "manifest")[0] ?? null;
}

export function normalizeManifestForAPI(
  root: ManifestSchema,
  value: unknown,
): unknown {
  return normalizeNode(root, root, value, "manifest");
}

function resolveSchema(root: ManifestSchema, input: ManifestSchema): ManifestSchema {
  if (!input.$ref) return input;
  const prefix = "#/$defs/";
  if (!input.$ref.startsWith(prefix)) return input;
  return root.$defs?.[input.$ref.slice(prefix.length)] ?? input;
}

function schemaType(schema: ManifestSchema): string | undefined {
  if (schema.type) return schema.type;
  if (
    schema.properties ||
    schema.required ||
    schema.additionalProperties !== undefined
  ) {
    return "object";
  }
  if (schema.items || schema.minItems !== undefined || schema.maxItems !== undefined) {
    return "array";
  }
  return undefined;
}

function objectValueSchema(
  root: ManifestSchema,
  input: ManifestSchema,
): ManifestSchema | null {
  let schema = resolveSchema(root, input);
  if (schemaType(schema) === "array" && schema.items) {
    schema = resolveSchema(root, schema.items);
  }
  return schemaType(schema) === "object" ? schema : null;
}

function contextSchema(
  root: ManifestSchema,
  context: string,
): ManifestSchema | null {
  return context === "root" ? root : findContextSchema(root, context);
}

function findContextSchema(
  root: ManifestSchema,
  target: string,
): ManifestSchema | null {
  const seen = new Set<ManifestSchema>();
  const visit = (input: ManifestSchema): ManifestSchema | null => {
    const schema = resolveSchema(root, input);
    if (seen.has(schema)) return null;
    seen.add(schema);
    for (const [key, property] of Object.entries(schema.properties ?? {})) {
      const child = objectValueSchema(root, property);
      if (!child) continue;
      if (key === target) return child;
      const nested = visit(child);
      if (nested) return nested;
    }
    return null;
  };
  return visit(root);
}

function scalarCompletionValues(
  root: ManifestSchema,
  input: ManifestSchema,
): unknown[] {
  const schema = resolveSchema(root, input);
  const result: unknown[] = [];
  if (schema.enum) result.push(...schema.enum);
  if (schema.const !== undefined) result.push(schema.const);
  if (schemaType(schema) === "boolean") result.push(true, false);
  for (const candidate of schema.oneOf ?? []) {
    result.push(...scalarCompletionValues(root, candidate));
  }
  return result;
}

function yamlScalar(value: unknown): string {
  if (typeof value === "boolean" || typeof value === "number") {
    return String(value);
  }
  if (value === null) return "null";
  const text = String(value);
  if (
    /^[A-Za-z0-9_./-]+$/u.test(text) &&
    !["true", "false", "null", "~"].includes(text.toLowerCase())
  ) {
    return text;
  }
  return JSON.stringify(text);
}

function validateNode(
  root: ManifestSchema,
  input: ManifestSchema,
  value: unknown,
  path: string,
): string[] {
  const schema = resolveSchema(root, input);

  if (schema.oneOf && schema.oneOf.length > 0) {
    const matches = schema.oneOf.some(
      (candidate) => validateNode(root, candidate, value, path).length === 0,
    );
    if (!matches) return [`${path} does not match the manifest schema`];
  }

  if (schema.enum && !schema.enum.some((candidate) => Object.is(candidate, value))) {
    return [`${path} must be one of ${schema.enum.map(String).join(", ")}`];
  }
  if (schema.const !== undefined && !Object.is(schema.const, value)) {
    return [`${path} must be ${String(schema.const)}`];
  }

  switch (schemaType(schema)) {
    case "object": {
      if (!isRecord(value)) return [`${path} must be a mapping`];
      for (const required of schema.required ?? []) {
        if (!(required in value)) return [`${path}.${required} is required`];
      }
      const properties = schema.properties ?? {};
      for (const [key, child] of Object.entries(value)) {
        const property = properties[key];
        if (property) {
          const issue = validateNode(root, property, child, `${path}.${key}`)[0];
          if (issue) return [issue];
          continue;
        }
        if (schema.additionalProperties === false) {
          return [`${path}.${key} is not a recognized field`];
        }
        if (isSchema(schema.additionalProperties)) {
          const issue = validateNode(
            root,
            schema.additionalProperties,
            child,
            `${path}.${key}`,
          )[0];
          if (issue) return [issue];
        }
      }
      break;
    }
    case "array": {
      if (!Array.isArray(value)) return [`${path} must be a list`];
      if (schema.minItems !== undefined && value.length < schema.minItems) {
        return [`${path} must contain at least ${schema.minItems} item(s)`];
      }
      if (schema.maxItems !== undefined && value.length > schema.maxItems) {
        return [`${path} must contain at most ${schema.maxItems} item(s)`];
      }
      if (schema.items) {
        for (let index = 0; index < value.length; index += 1) {
          const issue = validateNode(
            root,
            schema.items,
            value[index],
            `${path}[${index}]`,
          )[0];
          if (issue) return [issue];
        }
      }
      break;
    }
    case "string": {
      if (typeof value !== "string") return [`${path} must be a string`];
      if (schema.minLength !== undefined && value.length < schema.minLength) {
        return [`${path} is too short`];
      }
      if (schema.maxLength !== undefined && value.length > schema.maxLength) {
        return [`${path} is too long`];
      }
      if (schema.pattern && !new RegExp(schema.pattern, "u").test(value)) {
        return [`${path} has an invalid value`];
      }
      break;
    }
    case "integer": {
      if (typeof value !== "number" || !Number.isInteger(value)) {
        return [`${path} must be an integer`];
      }
      if (schema.minimum !== undefined && value < schema.minimum) {
        return [`${path} must be at least ${schema.minimum}`];
      }
      if (schema.maximum !== undefined && value > schema.maximum) {
        return [`${path} must be at most ${schema.maximum}`];
      }
      break;
    }
    case "boolean":
      if (typeof value !== "boolean") return [`${path} must be true or false`];
      break;
  }

  for (const constraint of schema.allOf ?? []) {
    const issue = validateNode(root, constraint, value, path)[0];
    if (issue) return [issue];
  }
  if (
    schema.if &&
    schema.then &&
    validateNode(root, schema.if, value, path).length === 0
  ) {
    const issue = validateNode(root, schema.then, value, path)[0];
    if (issue) return [issue];
  }
  if (schema.not && validateNode(root, schema.not, value, path).length === 0) {
    return [`${path} contains a combination that is not allowed`];
  }

  return [];
}

function normalizeNode(
  root: ManifestSchema,
  input: ManifestSchema,
  value: unknown,
  path: string,
): unknown {
  const schema = resolveSchema(root, input);
  const humanUnit = humanCanonicalUnit(root, schema);
  if (humanUnit === "nanoseconds" && typeof value === "string") {
    return durationNanoseconds(value, path);
  }
  if (humanUnit === "bytes" && typeof value === "string") {
    return byteSizeBytes(value, path);
  }

  if (schemaType(schema) === "array" && schema.items && Array.isArray(value)) {
    return value.map((item, index) =>
      normalizeNode(root, schema.items!, item, `${path}[${index}]`),
    );
  }
  if (schemaType(schema) === "object" && isRecord(value)) {
    const result: Record<string, unknown> = { ...value };
    for (const [key, property] of Object.entries(schema.properties ?? {})) {
      if (key in result) {
        result[key] = normalizeNode(root, property, result[key], `${path}.${key}`);
      }
    }
    return result;
  }
  return value;
}

function humanCanonicalUnit(
  root: ManifestSchema,
  schema: ManifestSchema,
): "nanoseconds" | "bytes" | null {
  if (!schema.oneOf) return null;
  for (const candidate of schema.oneOf) {
    const resolved = resolveSchema(root, candidate);
    if (resolved.type !== "integer") continue;
    if (resolved.description === "Nanoseconds.") return "nanoseconds";
    if (resolved.description === "Bytes.") return "bytes";
  }
  return null;
}

function durationNanoseconds(value: string, field: string): number {
  if (value === "0" || value === "") return 0;
  const units: Record<string, number> = {
    ns: 1,
    us: 1_000,
    "µs": 1_000,
    "μs": 1_000,
    ms: 1_000_000,
    s: 1_000_000_000,
    m: 60_000_000_000,
    h: 3_600_000_000_000,
  };
  const pattern = /(\d+(?:\.\d+)?)(ns|us|µs|μs|ms|s|m|h)/gu;
  let total = 0;
  let consumed = 0;
  let match: RegExpExecArray | null;
  while ((match = pattern.exec(value)) !== null) {
    if (match.index !== consumed) {
      throw new Error(`${field} has an invalid duration: ${value}`);
    }
    total += Number(match[1]) * units[match[2]];
    consumed = pattern.lastIndex;
  }
  if (consumed !== value.length || consumed === 0) {
    throw new Error(`${field} has an invalid duration: ${value}`);
  }
  return Math.round(total);
}

function byteSizeBytes(value: string, field: string): number {
  const match = value.trim().match(/^([0-9]+(?:\.[0-9]+)?)\s*([A-Za-z]*)$/);
  if (!match) {
    throw new Error(`${field} has an invalid byte size: ${value}`);
  }
  const units: Record<string, number> = {
    "": 1,
    b: 1,
    kb: 1_000,
    mb: 1_000_000,
    gb: 1_000_000_000,
    tb: 1_000_000_000_000,
    ki: 2 ** 10,
    kib: 2 ** 10,
    mi: 2 ** 20,
    mib: 2 ** 20,
    gi: 2 ** 30,
    gib: 2 ** 30,
    ti: 2 ** 40,
    tib: 2 ** 40,
  };
  const multiplier = units[match[2].toLowerCase()];
  if (multiplier === undefined) {
    throw new Error(`${field} has an invalid byte-size unit: ${match[2]}`);
  }
  const bytes = Number(match[1]) * multiplier;
  if (!Number.isSafeInteger(Math.round(bytes)) || bytes < 0) {
    throw new Error(`${field} is too large`);
  }
  return Math.round(bytes);
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === "object" && !Array.isArray(value);
}

function isSchema(value: boolean | ManifestSchema | undefined): value is ManifestSchema {
  return !!value && typeof value === "object";
}
