import type { Hono } from 'hono';
import type { MediaMTXConfig } from '../manager/mtx';

export interface Module<Id extends string, Metadata extends Record<string, unknown>, Handler extends Hono> {
  id: Id;
  path: string;
  metadata: Metadata;
  config: MediaMTXConfig;
  handler: Handler;
}

export type AnyModule = Module<string, Record<string, unknown>, Hono>;

export const module = <Id extends string, Metadata extends Record<string, unknown>, Handler extends Hono>(module: Module<Id, Metadata, Handler>) => {
  return module;
};
