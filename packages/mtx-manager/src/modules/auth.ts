import { Hono } from 'hono';
import { describeRoute, resolver, validator as zValidator } from 'hono-openapi';
import { z } from 'zod';
import { type Helpers } from '../internal/module/helpers';
import { module } from '../internal/module/module';

export const isLoopback = (ip: string) => {
  return ip === '127.0.0.1' || ip === '::1';
};

export const CheckParamsSchema = z.object({
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

export const AuthMetadataSchema = z.object({});

export const AuthCheckOkSchema = z.object({
  result: z.boolean()
});

export const AuthErrorSchema = z.object({
  message: z.string()
});

type CheckParams = z.infer<typeof CheckParamsSchema>;

interface CreateAuthModuleOptions {
  check: (params: CheckParams) => Promise<boolean>;
}

export const createAuthModule =
  ({ check }: CreateAuthModuleOptions) =>
  async (helpers: Helpers) => {
    const handler = new Hono();

    handler.post(
      '/check',
      describeRoute({
        operationId: 'checkAuthentication',
        description: 'MediaMTX HTTP authentication callback.',
        responses: {
          200: {
            description: 'Allowed or denied for the given credentials.',
            content: { 'application/json': { schema: resolver(AuthCheckOkSchema) } }
          },
          400: {
            description: 'Invalid request body',
            content: { 'application/json': { schema: resolver(AuthErrorSchema) } }
          },
          401: {
            description: 'Unauthorized',
            content: { 'application/json': { schema: resolver(AuthErrorSchema) } }
          }
        }
      }),
      zValidator('json', CheckParamsSchema),
      async c => {
        const params = c.req.valid('json');
        const result = await check(params);
        if (!result) return c.json({ message: 'Unauthorized' }, 401);
        return c.json({ result }, 200);
      }
    );

    return module({
      id: 'auth',
      path: '/auth',
      metadata: {},
      metadataSchema: AuthMetadataSchema,
      config: {
        authMethod: 'http',
        authHTTPAddress: helpers.makeUrl('/auth/check')
      },
      handler
    });
  };
