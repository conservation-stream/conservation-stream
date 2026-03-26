import { join } from 'node:path';

export const makeUrl =
  (origin: string, prefix: string) =>
  (path: string, { params, protocol }: { params?: URLSearchParams; protocol?: 'ws' | 'wss' | 'http' | 'https' } = {}) => {
    const base = new URL(prefix, origin);
    const cloned = new URL(base);
    const pathname = join(base.pathname, path);
    cloned.pathname = pathname;

    if (params) {
      for (const [key, value] of params.entries()) {
        cloned.searchParams.set(key, value);
      }
    }

    if (protocol) {
      cloned.protocol = protocol;
    }

    return cloned.toString();
  };

export interface Helpers {
  makeUrl: (path: string, params?: { params?: URLSearchParams; protocol?: 'ws' | 'wss' | 'http' | 'https' }) => string;
}
