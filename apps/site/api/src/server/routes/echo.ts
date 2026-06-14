import { sValidator } from '@hono/standard-validator';
import { Hono } from 'hono';
import { z } from 'zod';

const echoQuerySchema = z.object({
	message: z.string().min(1),
});

export const echo: Hono = new Hono()
	.get('/', sValidator('query', echoQuerySchema), (c) => {
		const { message } = c.req.valid('query');

		return c.json({
			message,
		});
	})
	.post('/', sValidator('json', echoQuerySchema), (c) => {
		const { message } = c.req.valid('json');

		return c.json({
			message,
		});
	});
