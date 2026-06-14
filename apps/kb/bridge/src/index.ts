import { serve } from '@hono/node-server';
import { Hono } from 'hono';
import { logger } from 'hono/logger';
import { webhook } from './webhook/route.ts';

const app = new Hono();

app.use(logger());
app.route('/webhook', webhook);

serve({ ...app, port: 3004 }, info => {
  console.log(`Server is running on ${info.address}${info.port}`);
});
