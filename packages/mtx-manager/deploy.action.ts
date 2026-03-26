import { deploy } from '@conservation-stream/internal-actions';
import { mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { $ } from 'zx';
import type { Payload } from './build.action.ts';

type Artifacts = 'package';

class TemporaryHandle {
  public readonly path: string;

  private constructor(path: string) {
    this.path = path;
  }

  static async create(prefix: string) {
    return new TemporaryHandle(await mkdtemp(path.join(tmpdir(), prefix)));
  }

  async dispose() {
    await rm(this.path, { recursive: true, force: true });
  }
}

const getBuild = (builds: Payload[]) => {
  const [build, ...rest] = builds;
  if (!build) {
    throw new Error('No package build found');
  }

  if (rest.length > 0) {
    throw new Error('Expected exactly one package build');
  }

  return build;
};

const RequiredSecrets = z.object({
  NPM_TOKEN: z.string()
});

await deploy<Payload, Artifacts>(async env => {
  const secrets = RequiredSecrets.parse(env.SECRETS);
  const build = getBuild(env.build);
  const packageFile = path.join(env.artifacts.package, build.fileName);
  const temp = await TemporaryHandle.create('actions-mtx-manager-npm-');
  const npmrcPath = path.join(temp.path, '.npmrc');

  try {
    await writeFile(npmrcPath, ['registry=https://registry.npmjs.org/', '//registry.npmjs.org/:_authToken=' + secrets.NPM_TOKEN, 'always-auth=true', ''].join('\n'));

    console.log(`Publishing ${build.packageName}@${build.version}`);
    console.log(`Tarball: ${packageFile}`);

    await $({
      cwd: env.GITHUB_WORKSPACE,
      env: {
        ...process.env,
        NODE_AUTH_TOKEN: secrets.NPM_TOKEN,
        NPM_CONFIG_USERCONFIG: npmrcPath
      }
    })`vp pm publish ${packageFile} --access public --no-git-checks -- --provenance`;
  } finally {
    await temp.dispose();
  }
});
