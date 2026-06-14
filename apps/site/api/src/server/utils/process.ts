interface CachedProcessOptions {
  /**
   * Time to live for the cached process, in milliseconds.
   */
  ttl: number;
}

interface GetOptions {
  /**
   * If true, the process will be re-run even if the data is still fresh.
   */
  wait: boolean;
}

export abstract class CachedProcess<T> {
  abstract get(options?: GetOptions): Promise<T>;
  abstract clear(): void;
}

export class MemoryCachedProcess<T> implements CachedProcess<T> {
  private process?: Promise<T>;
  private result?: { data: T; expires: number };

  private fn: () => Promise<T>;

  private options: CachedProcessOptions;
  constructor(fn: () => Promise<T>, options: CachedProcessOptions) {
    this.fn = fn;
    this.options = options;
  }

  async get(options?: GetOptions): Promise<T> {
    const forceRefresh = options?.wait ?? false;

    if (!this.result && !this.process) {
      this.process = this.fn()
        .then(data => {
          this.result = {
            data: data,
            expires: Date.now() + this.options.ttl
          };
          return data;
        })
        .finally(() => {
          delete this.process;
        });

      return await this.process;
    }

    if (!this.result && this.process) {
      return await this.process;
    }

    if (!this.result) throw new Error(`CachedProcess: This is an undefined state. Data should always be defined at the point.`);

    if ((!this.process && this.result.expires < Date.now()) || forceRefresh) {
      this.process = this.fn()
        .then(data => {
          this.result = {
            data: data,
            expires: Date.now() + this.options.ttl
          };
          return data;
        })
        .finally(() => {
          delete this.process;
        });
    }

    if (forceRefresh) {
      if (!this.process) {
        throw new Error('CachedProcess: This is an undefined state. Data should always be defined at the point');
      }

      return await this.process;
    }

    return this.result.data;
  }

  clear(): void {
    this.result = undefined;
    this.process = undefined;
  }
}
