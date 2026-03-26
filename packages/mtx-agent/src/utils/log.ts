import { existsSync } from 'node:fs';
import { writeFile } from 'node:fs/promises';
import type { Interface } from 'node:readline';
import { createInterface } from 'node:readline/promises';
import { createReadStream as createLineReadStream } from 'tail-file-stream';
import type { z } from 'zod';
import { AsyncBatchQueue } from './batch.ts';

const MAX_BATCH_SIZE = 100;
const MAX_BATCH_WAIT_MS = 500;

export class Log<T extends z.ZodType<unknown, string>> {
  private readonly rl: Interface;
  private readonly path: string;
  private readonly batches = new AsyncBatchQueue<z.output<T>>(MAX_BATCH_SIZE, MAX_BATCH_WAIT_MS);
  private readonly pump: Promise<void>;
  private disposed = false;

  constructor(path: string, parser: T, { signal }: { signal: AbortSignal }) {
    this.path = path;
    const stream = createLineReadStream(path, { autoWatch: true });
    this.rl = createInterface({ input: stream, crlfDelay: Infinity });
    this.pump = this.readLines(parser);

    signal.addEventListener('abort', () => {
      void this[Symbol.asyncDispose]();
    });
  }

  static async create<T extends z.ZodType<unknown, string>>(path: string, parser: T, { signal }: { signal: AbortSignal }) {
    if (!existsSync(path)) await writeFile(path, '', 'utf8');
    return new Log(path, parser, { signal });
  }

  async [Symbol.asyncDispose]() {
    if (this.disposed) return;
    this.disposed = true;
    this.rl.close();
    this.batches.close();
    await this.pump;
    await writeFile(this.path, '', 'utf8');
  }

  async *[Symbol.asyncIterator]() {
    yield* this.batches;
    await this.pump;
  }

  private async readLines(parser: T) {
    try {
      for await (const line of this.rl) {
        this.batches.push(parser.parse(line));
      }
    } finally {
      this.batches.close();
    }
  }
}
