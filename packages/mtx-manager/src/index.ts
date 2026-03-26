import { deepmerge } from 'deepmerge-ts';
import { Hono } from 'hono';
import { upgradeWebSocket } from 'hono/cloudflare-workers';
import type { MediaMTXConfig } from './internal/manager/mtx';
import { makeUrl, type Helpers } from './internal/module/helpers';
import type { AnyModule } from './internal/module/module';

export * from './internal/manager/mtx';
export * from './modules/auth';
export * from './modules/logs';
export * from './modules/recording';

export type AnyModuleFactory = (helpers: Helpers) => Promise<AnyModule>;

type ResolvedModule<F> = F extends (...args: any[]) => Promise<infer M> ? M : never;

type MetadataMap<Factories extends readonly AnyModuleFactory[]> = {
  [F in Factories[number] as ResolvedModule<F> extends { id: infer Id extends string } ? Id : never]: ResolvedModule<F> extends { metadata: infer M } ? M : never;
};

export type Metadata<T> = T extends readonly AnyModuleFactory[] ? MetadataMap<T> : T extends { metadata: infer M } ? M : never;

type Extensions<Factories extends readonly AnyModuleFactory[]> = {
  [F in Factories[number] as ResolvedModule<F> extends { id: infer Id extends string } ? Id : never]: Omit<ResolvedModule<F>, keyof AnyModule>;
};

interface MinimalWebSocketLike {
  send: (source: string) => void;
}

export type ModuleExtensions<T> = Omit<T, 'route' | 'metadata'>;

export type DeepRpc<T> = {
  [K in keyof T]: T[K] extends (...args: infer A) => infer R ? (...args: A) => R extends Promise<any> ? R : Promise<R> : DeepRpc<T[K]>;
};

export function createManagerProxy<T>(dispatch: (action: string[], args: unknown[]) => Promise<unknown>): DeepRpc<T> {
  function build(path: string[]): any {
    return new Proxy(() => {}, {
      get(_, prop: string) {
        return build([...path, prop]);
      },
      apply(_, __, args) {
        return dispatch(path, args);
      }
    });
  }
  return build([]);
}

export async function serveModuleHandlers<const Factories extends readonly AnyModuleFactory[]>(origin: string, prefix: string, config: MediaMTXConfig = {}, factories: Factories) {
  const helpers = { makeUrl: makeUrl(origin, prefix) };
  const hono = new Hono();
  const websockets = new Set<MinimalWebSocketLike>();
  const metadata = {} as Record<string, Record<string, unknown>>;
  const extensions: Record<string, unknown> = {};

  for (const factory of factories) {
    const { id, path, metadata: meta, config: modConfig, handler, ...rest } = await factory(helpers);
    metadata[id] = meta;
    config = deepmerge(config, modConfig);
    hono.route(path, handler);
    extensions[id] = rest;
  }

  hono.get(
    '/',
    upgradeWebSocket(_ => {
      return {
        onMessage: (_, websocket) => {
          websockets.add(websocket);
          websocket.send(JSON.stringify({ type: 'metadata', metadata }));
          websocket.send(JSON.stringify({ type: 'config', location: '/config/mediamtx.yml', config }));
        },
        onClose: (_, websocket) => {
          websockets.delete(websocket);
        }
      };
    })
  );

  return { route: hono, metadata, ...extensions } as { route: Hono; metadata: MetadataMap<Factories> } & Extensions<Factories>;
}
