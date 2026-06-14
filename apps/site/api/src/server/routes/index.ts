import { S3Client } from '@aws-sdk/client-s3';
import {
  createAuthModule,
  createLogModule,
  createRecordingModule,
  isLoopback,
  type Metadata,
  serveModuleHandlers
} from '@conservation-stream/mtx-manager';
import { DurableObject } from 'cloudflare:workers';
import { Hono } from 'hono';
import { cors } from 'hono/cors';
import { createBetterAuth } from '../services/auth';
import { createEnvironment, withEnvironment } from '../utils/env/env';
import { echo } from './echo';
import { health } from './health';

const s3 = new S3Client({
  endpoint: 'https://0def86f295b15a1a2d012774b38b04f9.r2.cloudflarestorage.com',
  credentials: {
    accessKeyId: '627f248fc4a639e3dcfe7b7ca3e661a1',
    secretAccessKey: '82522e2b6c4679898aed394c27942e6f01322749790c494f1197eb442a76b4df'
  },
  region: 'auto'
});

/*

rtsp: true
webrtc: true
rtmp: true

paths:
  stream:
    source: rtsp://root:kadfex-jarkuf-1Zobdu@192.168.4.36/axis-media/media.amp?camera=1&videoframeskipmode=empty&videozprofile=classic&resolution=1280x720&fps=50&audiocodec=aac&audiosamplerate=16000&audiobitrate=32000&timestamp=0&videocodec=h264&h264profile=baseline
    runOnReady: >
      /bin/sh -lc "cstream publish --in rtsp://127.0.0.1:8554/stream --out rtsp://127.0.0.1:8554/dynamic_stream --preset twitch --debug"
    runOnReadyRestart: yes
  ~^dynamic_:
    runOnReady: >
      /bin/sh -lc "cstream forward --in rtsp://127.0.0.1:8554/$MTX_PATH --out https://customer-51r7ggqg830zbmef.cloudflarestream.com/afe378616a92a11189167d163f17039ck1a44b550df472650ee84d734002ba871/webRTC/publish --debug"
    runOnReadyRestart: yes


*/

import type { Helpers, PathConfig } from '@conservation-stream/mtx-manager';
import { module } from '@conservation-stream/mtx-manager';

interface MinimalWebSocketLike {
  send: (source: string) => void;
}

import { Cloudflare } from 'cloudflare';

const cloudflare = new Cloudflare();

import assert from 'assert';
import { z } from 'zod';
const Meta = z.object({
  mediamtx_path: z.string()
});
type Meta = z.infer<typeof Meta>;

const getOrCreateLiveInput = async (path: string, inputs: Cloudflare.Stream.LiveInput[]) => {
  if (!process.env.CLOUDFLARE_ACCOUNT_ID) throw new Error('CLOUDFLARE_ACCOUNT_ID is not set');
  const input = inputs.find(input => {
    const parsed = Meta.safeParse(input.meta);
    if (!parsed.success) return false;
    return parsed.data.mediamtx_path === path;
  });

  if (input) {
    console.log(`Found existing live input for path ${path}`);
    assert(input.uid, 'Live input UID is required');
    return await cloudflare.stream.liveInputs.get(input.uid, { account_id: process.env.CLOUDFLARE_ACCOUNT_ID });
  }
  console.log(`No existing live input found for path ${path}, creating new one`);

  const newInput = await cloudflare.stream.liveInputs.create({
    account_id: process.env.CLOUDFLARE_ACCOUNT_ID,
    recording: { mode: 'off' },
    meta: {
      name: `${path} (dynamic)`,
      mediamtx_path: path
    }
  });
  return newInput;
};

const createDynamicModule = (paths: Record<string, PathConfig>) => async (helpers: Helpers) => {
  const handler = new Hono();
  const websockets = new Map<string, { id: string; websocket: MinimalWebSocketLike }>();

  handler.get(
    '/bitrate/:path',
    upgradeWebSocket(c => {
      const id = crypto.randomUUID();
      const path = c.req.param('path');
      console.log(`Agent ${id} attempting to connect to bitrate endpoint for path ${path}`);
      if (!path) throw new Error('Path is required');
      return {
        onMessage: (_, websocket) => {
          const existingWebsocketForPath = websockets.get(path);
          if (existingWebsocketForPath && existingWebsocketForPath.id !== id) {
            websocket.close(1008, 'An agent is already connected to service this request.');
            return;
          }

          websockets.set(path, { id, websocket });
        },
        onClose: (_, websocket) => {
          const existingWebsocketForPath = websockets.get(path);
          if (!existingWebsocketForPath) return;

          if (existingWebsocketForPath.id === id) {
            websockets.delete(path);
          }
        }
      };
    })
  );

  handler.post('/bitrate/:path', async c => {
    const path = c.req.param('path');
    if (!path) throw new Error('Path is required');
    const body = await c.req.json();
    const bitrate = body.bitrate;
    if (!bitrate) throw new Error('Bitrate is required');
    const websocket = websockets.get(path);
    if (!websocket) throw new Error('Websocket not found');
    websocket.websocket.send(JSON.stringify({ type: 'bitrate', bitrate }));
    return c.json({ success: true });
  });

  const pathsWithDynamic: Record<string, PathConfig> = {};
  const playbackUrls: Record<string, string> = {};

  if (!process.env.CLOUDFLARE_ACCOUNT_ID) throw new Error('CLOUDFLARE_ACCOUNT_ID is not set');
  const inputs = await cloudflare.stream.liveInputs.list({ account_id: process.env.CLOUDFLARE_ACCOUNT_ID });

  for (const [path, config] of Object.entries(paths)) {
    const input = await getOrCreateLiveInput(path, (inputs as Cloudflare.Stream.LiveInput[]) ?? []);
    const publishUrl = input.webRTC?.url;
    const playbackUrl = input.webRTCPlayback?.url;
    if (!publishUrl || !playbackUrl) throw new Error(`Failed to get publish or playback URL for ${path}`);
    playbackUrls[path] = playbackUrl;

    pathsWithDynamic[path] = {
      ...config,
      runOnReady: `cstream publish --in rtsp://127.0.0.1:8554/${path} --out rtsp://127.0.0.1:8554/dynamic_${path} --base-bitrate 100 --height 720 --width 1280 --rate 30/1 --dynamic ${helpers.makeUrl(`/dynamic/bitrate/${path}`, { protocol: 'ws' })} --debug`,
      runOnReadyRestart: true
    };

    pathsWithDynamic[`dynamic_${path}`] = {
      runOnReady: `cstream forward --in rtsp://127.0.0.1:8554/dynamic_${path} --out ${publishUrl} --debug`,
      runOnReadyRestart: true
    };
  }

  return {
    ...module({
      id: 'dynamic',
      path: '/dynamic',
      handler,
      metadata: {
        links: {
          playback: playbackUrls,
          bitrate: {
            post: helpers.makeUrl('/dynamic/bitrate/:path')
          }
        }
      },
      metadataSchema: z.object({
        links: z.object({
          playback: z.record(z.string(), z.string()),
          bitrate: z.object({
            post: z.string()
          })
        })
      }),
      config: {
        paths: pathsWithDynamic
      }
    })
  };
};

function createMTX() {
  return serveModuleHandlers({
    origin: 'http://host.docker.internal:8787',
    prefix: '/api/mtx',
    secret: 'secret',
    config: {
      paths: {}
    },
    factories: [
      createDynamicModule({
        fox: {
          rtspTransport: 'tcp',
          source:
            'rtsp://root:birds12345@192.168.100.155/axis-media/media.amp?camera=1&videoframeskipmode=empty&videozprofile=classic&resolution=1280x720&fps=30&audio=0&timestamp=0&videocodec=h264&h264profile=baseline'
        }
      }),
      createAuthModule({
        check: async params => {
          if (isLoopback(params.ip)) return true;
          return false;
        }
      }),
      createLogModule({
        logFile: '/logs/stream.log',
        logLevel: 'info',
        onLogs: logs => {
          // console.log(logs);
        }
      }),
      createRecordingModule({
        ttl: '14d',
        directory: '/recordings',
        pathsToRecord: ['fox', 'me']
      })
    ]
  });
}

type MTX = Awaited<ReturnType<typeof createMTX>>;

export type MTXMetadata = Metadata<MTX>;

export class MTXManager extends DurableObject {
  private mtx!: MTX;

  constructor(ctx: DurableObjectState, env: Env) {
    super(ctx, env);
    void this.ctx.blockConcurrencyWhile(async () => {
      this.mtx = await createMTX();
    });
  }

  async fetch(request: Request): Promise<Response> {
    const url = new URL(request.url);
    url.pathname = url.pathname.replace(/^\/api\/mtx/, '') || '/';
    return this.mtx.route.fetch(new Request(url, request));
  }

  async dispatch(action: string[], args: unknown[]): Promise<unknown> {
    let target: any = this.mtx;
    for (const key of action) target = target[key];
    return target(...args);
  }
}

const r = new Hono().on(['POST', 'GET'], '/*', async c => {
  const auth = await createBetterAuth();
  return auth.handler(c.req.raw);
});

const app = new Hono<{ Bindings: Env }>();

app.use('*', cors({ origin: 'http://localhost:5173', credentials: true }));
app.use(async (c, next) => {
  const environment = await createEnvironment(c.env);
  return withEnvironment(environment, () => next());
});
app.route('/api/health', health);
app.route('/api/echo', echo);
app.route('/api/auth', r);
app.all('/api/mtx/*', async c => {
  const stub = c.env.MTX_MANAGER.getByName('singleton');
  const request = forwardWithoutRoutePrefix(c);
  return stub.fetch(request);
});

export default app;
export type API = typeof app;

import console from 'console';
import type { Context } from 'hono';
import { upgradeWebSocket } from 'hono/cloudflare-workers';
import { routePath } from 'hono/route';

export function forwardWithoutRoutePrefix(context: Context) {
  const wildcard = routePath(context, 0);
  const rootRoute = routePath(context, 1);

  const prefix = rootRoute.replace(wildcard, '');
  const suffix = context.req.path.replace(prefix, '');

  const url = new URL(context.req.raw.url);
  url.pathname = suffix;

  const copied = context.req.raw.clone();

  const init: RequestInit = {
    method: copied.method,
    headers: copied.headers,
    body: copied.body
  };

  return new Request(url, init);
}
