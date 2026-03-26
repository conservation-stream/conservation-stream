import type { MTXMetadata } from '@conservation-stream/site-api';
import { writeFile } from 'node:fs/promises';
import { stringify } from 'yaml';
import { connect } from './internal/connection.ts';
import { handleMetrics } from './internal/handlers/metrics.ts';
import { handleRecording } from './internal/handlers/recording.ts';

if (!process.env.MTX_MANAGER_URL) throw new Error('MTX_MANAGER_URL is not set');

console.log(`Connecting to ${process.env.MTX_MANAGER_URL}`);
await connect<MTXMetadata>(process.env.MTX_MANAGER_URL, {
  onConfig: async ({ config, location }) => {
    console.log(`Writing config to ${location}`);
    await writeFile(location, stringify(config), { encoding: 'utf-8' });
  },
  onMetadata: async (metadata, { signal }) => {
    const recordingQueueProcess = handleRecording(metadata.recording, signal);
    const metricsProcess = handleMetrics(metadata.metrics, signal);
    await Promise.allSettled([recordingQueueProcess, metricsProcess]);
  }
});
