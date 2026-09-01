declare module "js-yaml" {
  export interface DumpOptions {
    indent?: number;
    lineWidth?: number;
    noRefs?: boolean;
    sortKeys?: boolean;
  }

  export function load(input: string): unknown;
  export function dump(input: unknown, options?: DumpOptions): string;
}
