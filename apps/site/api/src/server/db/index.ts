import { drizzle } from 'drizzle-orm/postgres-js';
import postgres from 'postgres';

import * as schema from './schema';

export const createDatabaseClient = async (url: string) => {
  const client = postgres(url, { prepare: false });
  return drizzle(client, { schema });
}
