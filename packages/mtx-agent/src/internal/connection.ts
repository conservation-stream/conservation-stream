import type { MediaMTXConfig } from '@conservation-stream/mtx-manager';
import { WebSocket } from 'partysocket';

interface ConnectParams<Metadata> {
  onConfig: (config: { location: string; config: MediaMTXConfig }) => Promise<void>;
  onMetadata: (metadata: Metadata, { signal }: { signal: AbortSignal }) => Promise<void>;
}

export const connect = async <Metadata>(url: string, params: ConnectParams<Metadata>) => {
  const control = new WebSocket(url);

  control.addEventListener('open', () => {
    console.log(`Connected to ${url}`);
    control.send(JSON.stringify({ type: 'init' }));
  });

  control.addEventListener('error', error => {
    console.error('Error connecting to control', error);
  });

  let downstreamServicesAbortController = new AbortController();
  control.addEventListener('message', async event => {
    const data = JSON.parse(event.data);
    console.log('Received message from control', data);
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
