import { copy } from "./copy.fa";

// All Persian text lives in copy.fa.ts. `t()` keeps the historical flat keys
// used across the dashboard; new code should use copy.pages / copy.errors.
const fa = copy.ui;

export type StringKey = keyof typeof fa;

export function t(key: StringKey): string {
  return fa[key] || String(key);
}

export function ti(key: StringKey, params: Record<string, string | number>): string {
  return t(key).replace(/\{(\w+)\}/g, (_, name: string) => {
    const value = params[name];
    return value === undefined ? `{${name}}` : String(value);
  });
}

export default fa;
