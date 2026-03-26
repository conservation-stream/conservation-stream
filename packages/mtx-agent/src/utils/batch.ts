export class AsyncBatchQueue<T> {
  private pending: T[] = [];
  private readonly ready: T[][] = [];
  private timer: ReturnType<typeof setTimeout> | undefined;
  private waiter: (() => void) | undefined;
  private done = false;
  private readonly maxSize: number;
  private readonly maxWaitMs: number;

  constructor(maxSize: number, maxWaitMs: number) {
    this.maxSize = maxSize;
    this.maxWaitMs = maxWaitMs;
  }

  push(value: T) {
    if (this.done) return;
    this.pending.push(value);
    if (this.pending.length >= this.maxSize) this.flush();
    else this.resetTimer();
  }

  close() {
    if (this.done) return;
    this.done = true;
    this.flush();
    this.wake();
  }

  async *[Symbol.asyncIterator]() {
    while (true) {
      if (this.ready.length > 0) {
        yield this.ready.shift()!;
        continue;
      }
      if (this.done) break;
      await new Promise<void>(r => {
        this.waiter = r;
      });
    }
  }

  private resetTimer() {
    clearTimeout(this.timer);
    this.timer = setTimeout(() => this.flush(), this.maxWaitMs);
  }

  private flush() {
    clearTimeout(this.timer);
    this.timer = undefined;
    if (this.pending.length === 0) return;
    this.ready.push(this.pending);
    this.pending = [];
    this.wake();
  }

  private wake() {
    this.waiter?.();
    this.waiter = undefined;
  }
}
