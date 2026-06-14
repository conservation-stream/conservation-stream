const INTERVAL = 1_000;

export async function poll<T>(fn: () => Promise<T>, until: (result: T) => boolean, signal: AbortSignal = AbortSignal.timeout(120_000)): Promise<T> {
  while (!signal.aborted) {
    const result = await fn();
    if (until(result)) return result;
    await new Promise<void>((resolve, reject) => {
      const timer = setTimeout(resolve, INTERVAL);
      signal.addEventListener(
        'abort',
        () => {
          clearTimeout(timer);
          reject(signal.reason);
        },
        { once: true }
      );
    });
  }

  throw signal.reason;
}
