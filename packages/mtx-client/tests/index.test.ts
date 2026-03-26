import { test } from 'vite-plus/test';
import { configPathsList, pathsList, webrtcSessionsList } from '../src/mediamtx/sdk.gen.ts';

test('joins active paths to their reader participants', async () => {
  console.log(JSON.stringify(await pathsList(), null, 2));
  console.log(JSON.stringify(await configPathsList(), null, 2));
  console.log(JSON.stringify(await webrtcSessionsList(), null, 2));
});
