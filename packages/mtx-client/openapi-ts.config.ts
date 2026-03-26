import { defineConfig } from '@hey-api/openapi-ts';

export default defineConfig({
  input: 'https://raw.githubusercontent.com/bluenviron/mediamtx/refs/tags/v1.17.0/api/openapi.yaml',
  output: {
    entryFile: false,
    path: 'src/mediamtx'
  },
  plugins: ['@hey-api/client-fetch', 'zod', '@hey-api/sdk']
});
