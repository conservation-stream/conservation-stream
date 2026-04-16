import type { MediaMTXConfig } from '@conservation-stream/mtx-manager';
import { WebSocket } from 'partysocket';

interface ConnectParams<Metadata> {
  onConfig: (config: { location: string; config: MediaMTXConfig }) => Promise<void>;
  onMetadata: (metadata: Metadata, { signal }: { signal: AbortSignal }) => Promise<void>;
}

export const connect = async <Metadata>(options: { url: string; secret: string }, params: ConnectParams<Metadata>) => {
  const url = new URL(options.url);
  url.searchParams.set('secret', options.secret);
  console.log(`Connecting to ${url.toString()}`);
  const control = new WebSocket(url.toString());

  control.addEventListener('open', () => {
    console.log(`Connected to ${url.toString()}`);
    control.send(JSON.stringify({ type: 'init' }));
  });

  control.addEventListener('error', error => {
    console.error('Error connecting to control', error);
  });

  let downstreamServicesAbortController = new AbortController();
  control.addEventListener('message', async event => {
    const data = JSON.parse(event.data);
    switch (data.type) {
      case 'config':
        return params.onConfig(data);
      case 'metadata':
        downstreamServicesAbortController.abort();
        downstreamServicesAbortController = new AbortController();
        return params.onMetadata(data.metadata, { signal: downstreamServicesAbortController.signal });
    }
  });

  process.on('SIGINT', () => {
    downstreamServicesAbortController.abort();
    control.close();
  });
};
