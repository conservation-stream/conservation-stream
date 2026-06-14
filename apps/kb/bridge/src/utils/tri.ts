interface Tri {
  <T>(fn: () => Promise<T>): Promise<T | Error>;
  $type: <E extends Error>() => <T>(fn: () => Promise<T>) => Promise<T | E>;
}

export const tri: Tri = Object.assign(
  async <T>(fn: () => Promise<T>): Promise<T | Error> => {
    try {
      return await fn();
    } catch (error) {
      return Promise.resolve(error);
    }
  },
  {
    $type:
      <E extends Error>() =>
      async <T>(fn: () => Promise<T>): Promise<T | E> => {
        try {
          return await fn();
        } catch (error) {
          return error as E;
        }
      }
  } as Pick<Tri, '$type'>
);
