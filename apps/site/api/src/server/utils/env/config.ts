import z from 'zod';
import { createDatabaseClient } from '../../db';

export const config = z.object({
  something: z.string().optional()
});

export const services = async (env: Env & z.infer<typeof config>) => {
  const db = await createDatabaseClient(env.DATABASE.connectionString);
  return { db };
};
