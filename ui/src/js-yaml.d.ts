declare module "js-yaml" {
  export interface LoadOptions {
    schema?: unknown;
  }

  export interface DumpOptions {
    indent?: number;
    lineWidth?: number;
    noRefs?: boolean;
    sortKeys?: boolean;
  }

  export const CORE_SCHEMA: unknown;
  export function load(input: string, options?: LoadOptions): unknown;
  export function dump(input: unknown, options?: DumpOptions): string;
}
