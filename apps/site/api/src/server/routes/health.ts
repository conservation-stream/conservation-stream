import { Hono } from 'hono';

export const health = new Hono()
  .get('/', c => {
    return c.json({
      ok: true
    });
  })
  .get('/ready', c => {
    return c.json({
      ready: true
    });
  });
