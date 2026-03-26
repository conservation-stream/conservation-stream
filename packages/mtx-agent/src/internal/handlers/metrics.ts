import type { MTXMetadata } from '@conservation-stream/site-api';
import { existsSync } from 'node:fs';
import { writeFile } from 'node:fs/promises';
import { z } from 'zod';
import { Log } from '../../utils/log.ts';

const LogSchema = z
  .string()
  .transform(json => JSON.parse(json))
  .pipe(
    z.object({
      timestamp: z.string(),
      level: z.string(),
      message: z.string()
    })
  );

export const handleMetrics = async (metrics: MTXMetadata['metrics'], signal: AbortSignal) => {
  if (!existsSync(metrics.logFile)) {
    await writeFile(metrics.logFile, '', 'utf8');
  }
  await using logs = await Log.create(metrics.logFile, LogSchema, { signal });
  for await (const lines of logs) {
    if (signal.aborted) break;
    await fetch(metrics.links.logs, {
      method: 'POST',
      body: JSON.stringify(lines),
      headers: { 'Content-Type': 'application/json' }
    });
  }
};
