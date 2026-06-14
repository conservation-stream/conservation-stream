import { FetchError } from '../types/errors.ts';
import { tri } from './tri.ts';

const key = 'ol_api_QniOsZ5EaiuN5FNWKGk3tyxTW72lhRiW7qu9nY';
const headers = {
  Authorization: `Bearer ${key}`
};
const url = 'https://editor.conservation.stream/api';

export interface OutlineRedirect {
  url: string;
}

export const outline = async <T>(method: string, payload: unknown) => {
  const response = await tri(
    async () =>
      await fetch(`${url}/${method}`, {
        method: 'POST',
        headers: {
          ...headers,
          'Content-Type': 'application/json'
        },
        body: JSON.stringify(payload),
        redirect: 'manual'
      })
  );

  if (response instanceof Error) return response;
  if (response.status == 302) {
    const location = response.headers.get('Location');
    if (!location) return new FetchError(`Failed to fetch ${method}`, response.status, await response.text());
    return { url: location } as T;
  }
  if (!response.ok) return new FetchError(`Failed to fetch ${method}`, response.status, await response.text());

  return (await response.json()) as T;
};
