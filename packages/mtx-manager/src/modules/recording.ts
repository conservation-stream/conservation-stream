import { Hono } from 'hono';
import { describeRoute, resolver, validator as zValidator } from 'hono-openapi';
import { upgradeWebSocket } from 'hono/cloudflare-workers';
import { join } from 'node:path';
import { z } from 'zod';
import type { PathConfig } from '../internal/mediamtx/path';
import type { Helpers } from '../internal/module/helpers';
import { module } from '../internal/module/module';

interface RecordingModuleParams {
  /**
   * An absolute path of the directory that mediamtx should record videos to.
   * Ensure that this is writeable by the mediamtx instance.
   */
  directory: string;

  /**
   * How long should recordings be kept before being deleted, e.g. '120s' | '14d' | '28d'.
   */
  ttl: string;

  pathsToRecord: string[];
}

interface MinimalWebSocketLike {
  send: (source: string) => void;
}

export const RecordingRequestParams = z.object({
  path: z.string(),
  startDate: z.iso.datetime(),
  duration: z.number()
});

const UploadInfoParams = z.object({
  type: z.literal('s3'),
  signedUrl: z.string()
});

export const RecordingRequest = z.object({
  id: z.string(),
  params: RecordingRequestParams,
  storage: z.object({
    type: z.literal('s3'),
    signedUrl: z.string()
  }),
  completeUrl: z.string()
});

export type RecordingRequest = z.output<typeof RecordingRequest>;

export type RecordingRequestParams = z.output<typeof RecordingRequestParams>;
export type UploadInfoParams = z.output<typeof UploadInfoParams>;

const RecordingSuccessBodySchema = z.object({
  id: z.string(),
  status: z.literal('success')
});

const RecordingFailedBodySchema = z.object({
  id: z.string(),
  status: z.literal('error'),
  message: z.string()
});

const RecordingCompleteBodySchema = z.discriminatedUnion('status', [
  RecordingSuccessBodySchema,
  RecordingFailedBodySchema
]);

export const RecordingCompleteOkSchema = z.object({
  success: z.literal(true)
});

export const RecordingMetadataSchema = z.object({
  playbackAddress: z.string(),
  directory: z.string(),
  links: z.object({
    queue: z.string()
  })
});

export const createRecordingModule =
  ({ directory, ttl, pathsToRecord }: RecordingModuleParams) =>
  async (helpers: Helpers) => {
    const handler = new Hono();
    const requests = new Map<string, [() => void, (error: unknown) => void]>();
    // There can only be one connected agent at a time
    let agent: { id: string; websocket: MinimalWebSocketLike } | null = null;
    console.log(`Setting up recording module with directory: ${directory} and ttl: ${ttl}`);

    const completeUrl = helpers.makeUrl('/recordings/complete');
    const request = async (params: RecordingRequestParams & { uploadInfo: UploadInfoParams }) => {
      if (!agent) throw new Error('No agent is connected to service this request.');
      const id = crypto.randomUUID();
      const { promise, resolve, reject } = Promise.withResolvers<void>();
      try {
        requests.set(id, [resolve, reject]);
        const request: RecordingRequest = { id, params, storage: params.uploadInfo, completeUrl };
        agent.websocket.send(JSON.stringify({ type: 'upload', request }));
      } catch (error) {
        reject(error);
      }
      return await promise;
    };

    handler.post(
      '/complete',
      describeRoute({
        operationId: 'completeRecordingUpload',
        description: 'Called by the recording agent when an upload to object storage has finished.',
        responses: {
          200: {
            description: 'Completion recorded',
            content: { 'application/json': { schema: resolver(RecordingCompleteOkSchema) } }
          },
          400: {
            description: 'Invalid body'
          },
          404: {
            description: 'Unknown recording request id'
          }
        }
      }),
      zValidator('json', RecordingCompleteBodySchema),
      async c => {
        const status = c.req.valid('json');
        const resolvers = requests.get(status.id);
        if (!resolvers) {
          return c.json({ message: `Request with id ${status.id} not found` }, 404);
        }
        const [resolve, reject] = resolvers;

        if (status.status === 'success') {
          resolve();
        } else {
          reject(new Error(status.message));
        }

        return c.json({ success: true }, 200);
      }
    );

    handler.get(
      '/queue',
      describeRoute({
        operationId: 'connectToRecordingQueue',
        description:
          'WebSocket for a single recording agent; becomes active on first client message. Request payloads are not part of HTTP (use description only).',
        responses: {
          101: { description: 'Switching Protocols (WebSocket)' }
        }
      }),
      upgradeWebSocket(_ => {
        const id = crypto.randomUUID();
        console.log(`Agent ${id} attempting to connect to recording queue`);
        return {
          onMessage: (_, websocket) => {
            console.log(`Agent ${id} sent message to recording queue`);
            if (agent && agent.id !== id) {
              console.log(`Agent ${id} is not the active agent, closing connection`);
              websocket.close(1008, 'An agent is already connected to service this request.');
              return;
            }
            if (agent) console.log(`There is an existing agent: ${agent.id}`);
            console.log(`Agent ${id} is now the active agent`);
            agent = { id, websocket };
          },
          onClose: () => {
            if (agent && agent.id === id) {
              console.log(`Agent ${id} disconnected from recording queue`);
              agent = null;
            }
          }
        };
      })
    );

    const paths: Record<string, PathConfig> = {};

    for (const path of pathsToRecord) {
      paths[path] = { record: true };
    }

    return {
      ...module({
        id: 'recording',
        path: '/recordings',
        handler,
        metadata: {
          playbackAddress: ':9996',
          directory,
          links: {
            queue: helpers.makeUrl('/recordings/queue')
          }
        },
        metadataSchema: RecordingMetadataSchema,
        config: {
          playback: true,
          playbackAddress: ':9996',
          pathDefaults: {
            recordPath: join(directory, '/%path/%Y-%m-%d_%H-%M-%S-%f'),
            recordDeleteAfter: ttl
          },
          paths
        }
      }),
      request
    };
  };
