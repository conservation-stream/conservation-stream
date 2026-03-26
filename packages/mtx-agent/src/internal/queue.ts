import { WebSocket } from 'partysocket';
import { AsyncBatchQueue } from '../utils/batch.ts';

export class Queue extends WebSocket {
  private readonly messages = new AsyncBatchQueue<MessageEvent>(1, 0);

  constructor(url: string, { signal }: { signal: AbortSignal }) {
    super(url, [], { minReconnectionDelay: 10_000, maxReconnectionDelay: 360_000, reconnectionDelayGrowFactor: 2 });

    signal.addEventListener('abort', () => {
      this.close();
    });

    this.addEventListener('open', () => {
      this.send(JSON.stringify({ type: 'init' }));
      console.log('Connected to queue');
    });

    this.addEventListener('error', error => {
      console.error('Error connecting to queue', error);
    });

    this.addEventListener('close', event => {
      console.log(`Disconnected from queue: ${event.reason}`);
    });

    this.addEventListener('message', event => {
      this.messages.push(event);
    });
  }
  async *[Symbol.asyncIterator]() {
    for await (const [message] of this.messages) {
      yield message;
    }
  }
}
