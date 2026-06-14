import { deploy } from '@conservation-stream/internal-actions';
import { join } from 'node:path';
import { z } from 'zod';
import { $ } from 'zx';
import type { Payload } from './build.action.ts';

type Artifacts = never; // No artifacts for this build

const RequiredSecrets = z
  .string()
  .transform(value => JSON.parse(value))
  .pipe(
    z.object({
      PLANETSCALE_SERVICE_TOKEN_ID: z.string(),
      PLANETSCALE_SERVICE_TOKEN: z.string(),
      PULUMI_ACCESS_TOKEN: z.string(),
      CLOUDFLARE_API_TOKEN: z.string(),
      CLOUDFLARE_ACCOUNT_ID: z.string(),
      DIGITALOCEAN_TOKEN: z.string()
    })
  );

await deploy<Payload, Artifacts>(async env => {
  const secrets = RequiredSecrets.parse(env.SECRETS);
  if (!env.ENVIRONMENT) throw new Error('ENVIRONMENT is required');

  for (const build of env.build) {
    console.log(`Deploying ${build.arch} build:`);
    console.log(`digest: ${build.digest}`);
    console.log(`timestamp: ${build.timestamp.toISOString()}`);
  }

  await $({
    cwd: join(import.meta.dirname, 'infra'),
    env: {
      ...process.env,
      PLANETSCALE_SERVICE_TOKEN_ID: secrets.PLANETSCALE_SERVICE_TOKEN_ID,
      PLANETSCALE_SERVICE_TOKEN: secrets.PLANETSCALE_SERVICE_TOKEN,
      PULUMI_ACCESS_TOKEN: secrets.PULUMI_ACCESS_TOKEN,
      CLOUDFLARE_API_TOKEN: secrets.CLOUDFLARE_API_TOKEN,
      CLOUDFLARE_ACCOUNT_ID: secrets.CLOUDFLARE_ACCOUNT_ID,
      DIGITALOCEAN_TOKEN: secrets.DIGITALOCEAN_TOKEN
    }
  })`pulumi up --yes --stack ${env.ENVIRONMENT}`;
});
