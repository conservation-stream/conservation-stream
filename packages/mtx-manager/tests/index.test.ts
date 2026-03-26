import { Hono } from 'hono';
import { expect, test } from 'vite-plus/test';
import { createAuthModule } from '../src/modules/auth.ts';
import { createRecordingModule } from '../src/modules/recording.ts';

const isLoopback = (ip: string) => {
  return ip === '127.0.0.1' || ip === '::1';
};

test('fn', async () => {
  const auth = await createAuthModule({
    check: async params => {
      if (isLoopback(params.ip)) return true;
      return false;
    }
  });
  const recording = await createRecordingModule({
    storage: async () => ({
      type: 's3',
      getPresignedUploadURL: async () => 'https://example.com/upload'
    }),
    ttl: '14d',
    directory: '/mnt/recordings',
    pathsToRecord: ['garden']
  });
  const logs = await createLogModule({
    logFile: '/mnt/logs/stream.log',
    logLevel: 'info',
    onLogs: logs => {
      console.log(logs);
    }
  });
  const app = new Hono();
  const handler = await serveModuleHandlers('http://localhost:3000', '/prefix', [auth, recording, logs]);
  app.route('/prefix', handler);

  const response = await app.request('/prefix/metadata');
  const metadata = await response.json();
  console.log(metadata);

  expect(response.status).toBe(200);
});

import { access, appendFile, mkdtemp, rm, writeFile } from 'fs/promises';
import { tmpdir } from 'os';
import { join } from 'path';
import z from 'zod';
import { serveModuleHandlers } from '../src/index.ts';
import { createLogModule, Log } from '../src/modules/logs.ts';

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
  const result = await Promise.race([iterator.next().then(() => 'batch' as const), new Promise<'timeout'>(resolve => setTimeout(() => resolve('timeout'), timeoutMs))]);
  expect(result).toBe('timeout');
};

test('streams all existing lines in the file', async () => {
  await using temp = await TempLog.create();
  await writeFile(temp.path, 'test\ntest2\ntest3\n');
  await using file = await Log.create(temp.path, z.string());

  const iterator = file[Symbol.asyncIterator]();
  const lines = await collectLines(iterator, 3);

  expect(lines).toEqual(['test', 'test2', 'test3']);
});

test('streams existing and appended lines while open', async () => {
  await using temp = await TempLog.create();
  await writeFile(temp.path, 'existing-1\nexisting-2\n');
  await using file = await Log.create(temp.path, z.string());

  const iterator = file[Symbol.asyncIterator]();
  const existing = await collectLines(iterator, 2);
  expect(existing).toEqual(['existing-1', 'existing-2']);

  await appendFile(temp.path, 'appended-1\nappended-2\n');

  const appended = await collectLines(iterator, 2);
  expect(appended).toEqual(['appended-1', 'appended-2']);
});

test('cleans up file when log is out of scope', async () => {
  await using temp = await TempLog.create();
  await writeFile(temp.path, 'keep\n');

  {
    await using _file = await Log.create(temp.path, z.string());
  }

  await expect(access(temp.path)).rejects.toThrow();
});

test("doesn't stream a non-terminated trailing line", async () => {
  await using temp = await TempLog.create();
  await writeFile(temp.path, 'line-1\nline-2\ntruncated');
  await using file = await Log.create(temp.path, z.string());

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
  await using file = await Log.create(temp.path, Schema);

  const iterator = file[Symbol.asyncIterator]();
  const lines = await collectLines(iterator, 1);
  expect(lines).toEqual([{ timestamp: '2026-03-22T12:00:00.000Z', level: 'info', message: 'test' }]);
});
