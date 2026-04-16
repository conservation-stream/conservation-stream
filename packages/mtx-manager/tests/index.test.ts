import { Hono } from 'hono';
import { access, appendFile, mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { expect, test } from 'vite-plus/test';
import { z } from 'zod';
import { Log } from '../../mtx-agent/src/utils/log.ts';
import { serveModuleHandlers } from '../src/index.ts';
import { createAuthModule } from '../src/modules/auth.ts';
import { createLogModule } from '../src/modules/logs.ts';
import { createRecordingModule } from '../src/modules/recording.ts';

const isLoopback = (ip: string) => {
  return ip === '127.0.0.1' || ip === '::1';
};

function mountMtx() {
  const auth = createAuthModule({
    check: async params => {
      if (isLoopback(params.ip)) return true;
      return false;
    }
  });
  const recording = createRecordingModule({
    ttl: '14d',
    directory: '/mnt/recordings',
    pathsToRecord: ['garden']
  });
  const logs = createLogModule({
    logFile: '/mnt/logs/stream.log',
    logLevel: 'info',
    onLogs: () => {}
  });
  return serveModuleHandlers({
    origin: 'http://localhost:3000',
    prefix: '/prefix',
    secret: 'secret',
    config: {
      paths: {}
    },
    factories: [auth, recording, logs]
  });
}

test('serveModuleHandlers exposes metadata', async () => {
  const mtx = await mountMtx();
  const app = new Hono();
  app.route('/prefix', mtx.route);

  const response = await app.request('/prefix/metadata');
  const metadata = await response.json();

  expect(response.status).toBe(200);
  expect(metadata).toHaveProperty('auth');
  expect(metadata).toHaveProperty('recording');
  expect(metadata).toHaveProperty('metrics');
});

test('POST /auth/check rejects invalid body', async () => {
  const mtx = await mountMtx();
  const app = new Hono();
  app.route('/prefix', mtx.route);

  const response = await app.request('/prefix/auth/check', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({})
  });

  expect(response.status).toBe(400);
});

test('POST /metrics/logs rejects malformed payload when callback is set', async () => {
  const logs = createLogModule({
    logFile: '/mnt/logs/stream.log',
    logLevel: 'info',
    onLogs: () => {}
  });
  const mtx = await serveModuleHandlers({
    origin: 'http://localhost:3000',
    prefix: '/prefix',
    secret: 'secret',
    config: {
      paths: {}
    },
    factories: [logs]
  });
  const app = new Hono();
  app.route('/prefix', mtx.route);

  const response = await app.request('/prefix/metrics/logs', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify([{ bad: true }])
  });

  expect(response.status).toBe(400);
});

test('POST /metrics/paths rejects non-string array when callback is set', async () => {
  const logs = createLogModule({
    logFile: '/mnt/logs/stream.log',
    logLevel: 'info',
    onPaths: () => {}
  });
  const mtx = await serveModuleHandlers({
    origin: 'http://localhost:3000',
    prefix: '/prefix',
    secret: 'secret',
    config: {
      paths: {}
    },
    factories: [logs]
  });
  const app = new Hono();
  app.route('/prefix', mtx.route);

  const response = await app.request('/prefix/metrics/paths', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify([1, 2])
  });

  expect(response.status).toBe(400);
});

test('POST /recordings/complete returns 404 for unknown id', async () => {
  const recording = createRecordingModule({
    ttl: '14d',
    directory: '/mnt/recordings',
    pathsToRecord: ['garden']
  });
  const mtx = await serveModuleHandlers({
    origin: 'http://localhost:3000',
    prefix: '/prefix',
    secret: 'secret',
    config: {
      paths: {}
    },
    factories: [recording]
  });
  const app = new Hono();
  app.route('/prefix', mtx.route);

  const response = await app.request('/prefix/recordings/complete', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ id: 'nonexistent' })
  });

  expect(response.status).toBe(404);
});

test('GET /config returns 200', async () => {
  const mtx = await mountMtx();
  const app = new Hono();
  app.route('/prefix', mtx.route);

  const response = await app.request('/prefix/config');
  expect(response.status).toBe(200);
  const config = await response.json();
  expect(typeof config).toBe('object');
});

test('GET /openapi returns the generated MTX spec', async () => {
  const mtx = await mountMtx();
  const app = new Hono();
  app.route('/prefix', mtx.route);

  const response = await app.request('/prefix/openapi');
  const spec = await response.json();

  expect(response.status).toBe(200);
  expect(spec.info.title).toBe('MTX Manager API');
  expect(spec.paths).toHaveProperty('/metadata');
  expect(spec.paths).toHaveProperty('/config');
});

class TempLog {
  readonly path: string;
  private readonly directory: string;
  private constructor(directory: string) {
    this.directory = directory;
    this.path = join(directory, 'stream.log');
  }

  static async create() {
    const directory = await mkdtemp(join(tmpdir(), 'mtx-manager-log-test-'));
    return new TempLog(directory);
  }

  async [Symbol.asyncDispose]() {
    await rm(this.directory, { recursive: true, force: true });
  }
}

const nextBatch = async <T>(iterator: AsyncIterator<T[]>, timeoutMs = 2_000) => {
  const timeout = new Promise<never>((_, reject) => {
    setTimeout(() => reject(new Error('Timed out waiting for batch')), timeoutMs);
  });
  const result = await Promise.race([iterator.next(), timeout]);
  expect(result.done).toBe(false);
  return result.value;
};

const collectLines = async <T>(iterator: AsyncIterator<T[]>, count: number, timeoutMs = 2_000) => {
  const lines: T[] = [];
  while (lines.length < count) {
    const batch = await nextBatch(iterator, timeoutMs);
    lines.push(...batch);
  }
  return lines.slice(0, count);
};

const expectNoBatch = async <T>(iterator: AsyncIterator<T[]>, timeoutMs = 150) => {
  const result = await Promise.race([
    iterator.next().then(() => 'batch' as const),
    new Promise<'timeout'>(resolve => setTimeout(() => resolve('timeout'), timeoutMs))
  ]);
  expect(result).toBe('timeout');
};

test('streams all existing lines in the file', async () => {
  await using temp = await TempLog.create();
  await writeFile(temp.path, 'test\ntest2\ntest3\n');
  const signal = new AbortController().signal;
  await using file = await Log.create(temp.path, z.string(), { signal });

  const iterator = file[Symbol.asyncIterator]();
  const lines = await collectLines(iterator, 3);

  expect(lines).toEqual(['test', 'test2', 'test3']);
});

test('streams existing and appended lines while open', async () => {
  await using temp = await TempLog.create();
  await writeFile(temp.path, 'existing-1\nexisting-2\n');
  const signal = new AbortController().signal;
  await using file = await Log.create(temp.path, z.string(), { signal });

  const iterator = file[Symbol.asyncIterator]();
  const existing = await collectLines(iterator, 2);
  expect(existing).toEqual(['existing-1', 'existing-2']);

  await appendFile(temp.path, 'appended-1\nappended-2\n');

  const appended = await collectLines(iterator, 2);
  expect(appended).toEqual(['appended-1', 'appended-2']);
});

test('clears log file when Log is disposed', async () => {
  await using temp = await TempLog.create();
  await writeFile(temp.path, 'keep\n');

  {
    const signal = new AbortController().signal;
    await using _file = await Log.create(temp.path, z.string(), { signal });
  }

  await access(temp.path);
  const cleared = await readFile(temp.path, 'utf8');
  expect(cleared).toBe('');
});

test("doesn't stream a non-terminated trailing line", async () => {
  await using temp = await TempLog.create();
  await writeFile(temp.path, 'line-1\nline-2\ntruncated');
  const signal = new AbortController().signal;
  await using file = await Log.create(temp.path, z.string(), { signal });

  const iterator = file[Symbol.asyncIterator]();
  const lines = await collectLines(iterator, 2);
  expect(lines).toEqual(['line-1', 'line-2']);
  await expectNoBatch(iterator);
});

test('streams a single line that matches the schema', async () => {
  const Schema = z
    .string()
    .transform(json => JSON.parse(json))
    .pipe(
      z.object({
        timestamp: z.string(),
        level: z.string(),
        message: z.string()
      })
    );

  await using temp = await TempLog.create();
  await writeFile(temp.path, '{"timestamp":"2026-03-22T12:00:00.000Z","level":"info","message":"test"}\n');
  const signal = new AbortController().signal;
  await using file = await Log.create(temp.path, Schema, { signal });

  const iterator = file[Symbol.asyncIterator]();
  const lines = await collectLines(iterator, 1);
  expect(lines).toEqual([{ timestamp: '2026-03-22T12:00:00.000Z', level: 'info', message: 'test' }]);
});
