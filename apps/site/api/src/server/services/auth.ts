import { oauthProvider } from '@better-auth/oauth-provider';
import { betterAuth } from 'better-auth';
import { drizzleAdapter } from 'better-auth/adapters/drizzle';
import { admin, jwt, organization } from 'better-auth/plugins';
import { useEnvironment } from '../utils/env/env';
import { tri } from '../utils/tri';
import * as schema from '../db/schema';

interface CreateBetterAuthOptions {
  cli?: boolean

}

export const createBetterAuth = async (options: CreateBetterAuthOptions = {}) => {
    const database = await makeAdapter(options);
  return betterAuth({
        database,
        emailAndPassword: {
            enabled: true
        },
        plugins: [
            admin(),
            organization(),
            jwt(),
            // oauthProvider({
            //     loginPage: '/sign-in',
            //     consentPage: '/consent'
            // })
        ]
    });
};

export const makeAdapter = async (options: CreateBetterAuthOptions) => {
  const db = await tri(async () => {
      const { db } = useEnvironment();
      return db;
  })
  if (db instanceof Error && options.cli) {
      return drizzleAdapter({} as any, { provider: 'pg' })
  }
  if (db instanceof Error) return db;
  return drizzleAdapter(db, { provider: 'pg', schema })
}
