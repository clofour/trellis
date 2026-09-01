declare module "js-yaml" {
  export interface Schema {}

  export interface LoadOptions {
    schema?: Schema;
  }

  export interface DumpOptions {
    indent?: number;
    lineWidth?: number;
    noRefs?: boolean;
    sortKeys?: boolean;
  }

  export const CORE_SCHEMA: Schema;
  export function load(input: string, options?: LoadOptions): unknown;
  export function dump(input: unknown, options?: DumpOptions): string;
}
