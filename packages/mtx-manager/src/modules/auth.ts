import { Hono } from 'hono';
import { z } from 'zod';
import { type Helpers } from '../internal/module/helpers';
import { module } from '../internal/module/module';

export const isLoopback = (ip: string) => {
  return ip === '127.0.0.1' || ip === '::1';
};

const CheckParamsSchema = z.object({
  action: z.literal(['publish', 'read', 'playback']),
  path: z.string(),
  ip: z.string(),
  user: z.string(),
  query: z.string().optional(),
  password: z.string(),
  protocol: z.string(),
  id: z.string().nullable(),
  token: z.string()
});

type CheckParams = z.infer<typeof CheckParamsSchema>;

interface CreateAuthModuleOptions {
  check: (params: CheckParams) => Promise<boolean>;
}

export const createAuthModule =
  ({ check }: CreateAuthModuleOptions) =>
  async (helpers: Helpers) => {
    const handler = new Hono();

    handler.post('/check', async c => {
      const data = await c.req.json();
      console.log(data);
      const parsed = CheckParamsSchema.safeParse(data);
      if (!parsed.success) return c.json({ message: 'Invalid request' }, 400);
      const result = await check(parsed.data);

      if (!result) return c.json({ message: 'Unauthorized' }, 401);
      return c.json({ result }, 200);
    });

    return module({
      id: 'auth',
      path: '/auth',
      metadata: {},
      config: {
        authMethod: 'http',
        authHTTPAddress: helpers.makeUrl('/auth/check')
      },
      handler
    });
  };
