import { describeRoute, resolver, validator as zValidator } from 'hono-openapi';
import { Hono } from 'hono';
import { z } from 'zod';
import type { Helpers } from '../internal/module/helpers';
import { module } from '../internal/module/module';

export interface LogLine {
  timestamp: string;
  level: string;
  message: string;
}

export const LogLineSchema = z.object({
  timestamp: z.string(),
  level: z.string(),
  message: z.string()
}) satisfies z.ZodType<LogLine>;

export const LogLinesSchema = z.array(LogLineSchema);

export const PathsPayloadSchema = z.array(z.string());

export const MetricsAckSchema = z.object({
  ok: z.string()
});

export const MetricsMetadataSchema = z.object({
  links: z.object({
    logs: z.string(),
    paths: z.string()
  }),
  apiAddress: z.string(),
  logFile: z.string()
});

interface LogModuleParams {
  /**
   * The file to log to. Must be writeable by the mediamtx instance.
   * This is the file that will be used to store the logs.
   */
  logFile: string;
  logLevel: 'debug' | 'info' | 'warn' | 'error';
  /**
   * A callback that will be called with the logs.
   */
  onLogs?: (logs: LogLine[]) => void;
  /**
   * A callback that will be called with the paths.
   */
  onPaths?: (paths: string[]) => void;
}

export const createLogModule =
  ({ logFile, logLevel, onLogs, onPaths }: LogModuleParams) =>
  async (helpers: Helpers) => {
    const handler = new Hono();

    const logsRoute = describeRoute({
      operationId: 'submitMetricLogs',
      description: 'Receives structured log lines from MediaMTX metrics API.',
      responses: {
        200: {
          description: 'Acknowledgement',
          content: { 'application/json': { schema: resolver(MetricsAckSchema) } }
        }
      }
    });

    if (onLogs) {
      handler.post('/logs', logsRoute, zValidator('json', LogLinesSchema), async c => {
        onLogs(c.req.valid('json'));
        return c.json({ ok: 'Logs received' }, 200);
      });
    } else {
      handler.post('/logs', logsRoute, async c => {
        return c.json({ ok: 'No logs callback provided' }, 200);
      });
    }

    const pathsRoute = describeRoute({
      operationId: 'submitMetricPaths',
      description: 'Receives active path names from MediaMTX metrics API.',
      responses: {
        200: {
          description: 'Acknowledgement',
          content: { 'application/json': { schema: resolver(MetricsAckSchema) } }
        }
      }
    });

    if (onPaths) {
      handler.post('/paths', pathsRoute, zValidator('json', PathsPayloadSchema), async c => {
        onPaths(c.req.valid('json'));
        return c.json({ ok: 'Paths received' }, 200);
      });
    } else {
      handler.post('/paths', pathsRoute, async c => {
        return c.json({ ok: 'No paths callback provided' }, 200);
      });
    }

    return module({
      id: 'metrics',
      path: '/metrics',
      handler,
      metadata: {
        links: {
          logs: helpers.makeUrl('/metrics/logs'),
          paths: helpers.makeUrl('/metrics/paths')
        },
        apiAddress: ':9997',
        logFile
      },
      metadataSchema: MetricsMetadataSchema,
      config: {
        api: true,
        apiAddress: ':9997',

        logStructured: true,
        logDestinations: ['file', 'stdout'],
        logFile,
        logLevel
      }
    });
  };
