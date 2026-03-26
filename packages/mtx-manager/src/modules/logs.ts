import { Hono } from 'hono';
import type { Helpers } from '../internal/module/helpers';
import { module } from '../internal/module/module';

export interface LogLine {
  timestamp: string;
  level: string;
  message: string;
}

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

    handler.post('/logs', async c => {
      if (!onLogs) return c.json({ ok: 'No logs callback provided' }, 200);

      const lines = await c.req.json<LogLine[]>();
      onLogs(lines);
      return c.json({ ok: 'Logs received' }, 200);
    });

    handler.post('/paths', async c => {
      if (!onPaths) return c.json({ ok: 'No paths callback provided' }, 200);

      const paths = await c.req.json<string[]>();
      onPaths(paths);
      return c.json({ ok: 'Paths received' }, 200);
    });

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
