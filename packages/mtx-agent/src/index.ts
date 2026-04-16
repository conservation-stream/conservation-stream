import type { MTXMetadata } from '@conservation-stream/site-api';
import { writeFile } from 'node:fs/promises';
import { stringify } from 'yaml';
import { connect } from './internal/connection.ts';
import { handleMetrics } from './internal/handlers/metrics.ts';
import { handleRecording } from './internal/handlers/recording.ts';

if (!process.env.MTX_MANAGER_URL) throw new Error('MTX_MANAGER_URL is not set');
if (!process.env.MTX_AGENT_SECRET) throw new Error('MTX_AGENT_SECRET is not set');

await connect<MTXMetadata>(
  {
    url: process.env.MTX_MANAGER_URL,
    secret: process.env.MTX_AGENT_SECRET
  },
  {
    onConfig: async ({ config, location }) => {
      await writeFile(location, stringify(config), { encoding: 'utf-8' });
    },
    onMetadata: async (metadata, { signal }) => {
      const recordingQueueProcess = handleRecording(metadata.recording, signal);
      const metricsProcess = handleMetrics(metadata.metrics, signal);
      await Promise.allSettled([recordingQueueProcess, metricsProcess]);
    }
  }
);
