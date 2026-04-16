import { deepmerge } from 'deepmerge-ts';
import { Hono } from 'hono';
import { describeRoute, openAPIRouteHandler, resolver } from 'hono-openapi';
import { upgradeWebSocket } from 'hono/cloudflare-workers';
import { z } from 'zod';
import type { MediaMTXConfig } from './internal/manager/mtx';
import { GlobalConfig } from './internal/mediamtx/global';
import { makeUrl, type Helpers } from './internal/module/helpers';
import type { AnyModule } from './internal/module/module';

export * from './internal/manager/mtx';
export * from './internal/mediamtx/global';
export * from './internal/mediamtx/path';
export * from './internal/module/helpers';
export * from './internal/module/module';
export * from './modules/auth';
export * from './modules/logs';
export * from './modules/recording';

export type AnyModuleFactory = (helpers: Helpers) => Promise<AnyModule>;

type ResolvedModule<F> = F extends (...args: any[]) => Promise<infer M> ? M : never;

type MetadataMap<Factories extends readonly AnyModuleFactory[]> = {
  [F in Factories[number] as ResolvedModule<F> extends { id: infer Id extends string }
    ? Id
    : never]: ResolvedModule<F> extends { metadata: infer M } ? M : never;
};

export type Metadata<T> = T extends readonly AnyModuleFactory[]
  ? MetadataMap<T>
  : T extends { metadata: infer M }
    ? M
    : never;

type Extensions<Factories extends readonly AnyModuleFactory[]> = {
  [F in Factories[number] as ResolvedModule<F> extends { id: infer Id extends string } ? Id : never]: Omit<
    ResolvedModule<F>,
    keyof AnyModule
  >;
};

interface MinimalWebSocketLike {
  send: (source: string) => void;
}

export type ModuleExtensions<T> = Omit<T, 'route' | 'metadata'>;

export type DeepRpc<T> = {
  [K in keyof T]: T[K] extends (...args: infer A) => infer R
    ? (...args: A) => R extends Promise<any> ? R : Promise<R>
    : DeepRpc<T[K]>;
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

export async function serveModuleHandlers<const Factories extends readonly AnyModuleFactory[]>(params: {
  origin: string;
  prefix: string;
  secret: string;
  config: MediaMTXConfig;
  factories: Factories;
}) {
  const websockets = new Set<MinimalWebSocketLike>();
  const metadata = {} as Record<string, Record<string, unknown>>;
  const extensions: Record<string, unknown> = {};

  let config = params.config;

  const sessionToken = crypto.randomUUID();
  const helpers = { makeUrl: makeUrl(params.origin, params.prefix, sessionToken) };

  const hono = new Hono();

  let rolledUpMetadataSchema = z.object({}) as z.ZodObject<Record<string, z.ZodTypeAny>>;

  for (const factory of params.factories) {
    const {
      id,
      path,
      metadata: meta,
      metadataSchema: moduleMetadataSchema,
      config: modConfig,
      handler,
      ...rest
    } = await factory(helpers);
    metadata[id] = meta;
    const metaSchema = moduleMetadataSchema ?? z.record(z.string(), z.unknown());
    rolledUpMetadataSchema = rolledUpMetadataSchema.extend({
      [id]: metaSchema
    }) as z.ZodObject<Record<string, z.ZodTypeAny>>;
    config = merge(config, modConfig);
    hono.route(
      path,
      handler.use(async (c, next) => {
        const token = c.req.query('token');
        if (token !== sessionToken) {
          return c.json({ error: 'Unauthorized' }, 401);
        }
        await next();
      })
    );
    extensions[id] = rest;
  }

  hono.get(
    '/',
    describeRoute({
      operationId: 'connectToManager',
      description:
        'WebSocket: on first client message, sends JSON events containing aggregated module metadata and merged MediaMTX config (see payloads in handler; not fully described as HTTP response schemas).',
      responses: {
        101: { description: 'Switching Protocols (WebSocket)' }
      }
    }),
    upgradeWebSocket(c => {
      const secret = c.req.query('secret');
      if (secret !== params.secret) {
        return {
          close: true,
          code: 401,
          reason: 'Unauthorized'
        };
      }
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

  hono.get(
    '/metadata',
    describeRoute({
      operationId: 'getManagerMetadata',
      description: 'Aggregated module metadata keyed by module id (auth, metrics, recording, etc.).',
      responses: {
        200: {
          description: 'Metadata map',
          content: {
            'application/json': {
              schema: resolver(rolledUpMetadataSchema)
            }
          }
        }
      }
    }),
    async c => {
      const secret = c.req.query('secret');
      if (secret !== params.secret) {
        return c.json({ error: 'Unauthorized' }, 401);
      }
      return c.json(metadata, 200);
    }
  );

  hono.get(
    '/config',
    describeRoute({
      operationId: 'getManagerConfig',
      description: 'Merged MediaMTX configuration (global options); path-level entries may also be present at runtime.',
      responses: {
        200: {
          description: 'MediaMTX global config (Zod: GlobalConfig)',
          content: {
            'application/json': {
              schema: resolver(GlobalConfig)
            }
          }
        }
      }
    }),
    async c => {
      const secret = c.req.query('secret');
      if (secret !== params.secret) {
        return c.json({ error: 'Unauthorized' }, 401);
      }
      return c.json(config, 200);
    }
  );

  hono.get(
    '/openapi',
    openAPIRouteHandler(hono, {
      documentation: {
        info: {
          title: 'MTX Manager API',
          version: '1.0.0',
          description: 'OpenAPI for the MTX manager routes.'
        }
      },
      exclude: ['/openapi']
    })
  );

  return { route: hono, metadata, ...extensions } as {
    route: Hono;
    metadata: MetadataMap<Factories>;
  } & Extensions<Factories>;
}

const merge = <T>(a: T, b: T): T => {
  return deepmerge(a, b) as T;
};
