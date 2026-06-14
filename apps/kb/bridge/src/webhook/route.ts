import { Hono } from 'hono';
import { outlineWebhook } from './outline/route.ts';

export const webhook = new Hono();
webhook.route('/outline', outlineWebhook);
