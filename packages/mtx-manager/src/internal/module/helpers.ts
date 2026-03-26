import { join } from 'node:path';

export const makeUrl = (origin: string, prefix: string) => (path: string, params?: URLSearchParams) => {
  const base = new URL(prefix, origin);
  const cloned = new URL(base);
  const pathname = join(base.pathname, path);
  cloned.pathname = pathname;

  if (params) {
    for (const [key, value] of params.entries()) {
      cloned.searchParams.set(key, value);
    }
  }

  return cloned.toString();
};

export interface Helpers {
  makeUrl: (path: string, params?: URLSearchParams) => string;
}
