import { hc } from 'hono/client';

import type { API } from '../../../api/routes';
export const client = hc<API>('/', {});
