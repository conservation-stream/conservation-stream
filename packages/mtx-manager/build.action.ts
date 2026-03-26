import { build } from '@conservation-stream/internal-actions';
import { mkdtemp, readdir, readFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { $ } from 'zx';

export interface Payload {
  fileName: string;
  packageName: string;
  version: string;
}

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

const readPackageMetadata = async () => {
  const packageJsonPath = path.join(import.meta.dirname, 'package.json');
  const contents = await readFile(packageJsonPath, 'utf8');
  const pkg = JSON.parse(contents) as {
    name?: string;
    version?: string;
  };

  if (!pkg.name) {
    throw new Error('Package name is required');
  }

  if (!pkg.version) {
    throw new Error('Package version is required');
  }

  return {
    packageName: pkg.name,
    version: pkg.version
  };
};

await build<Payload, Artifacts>(async () => {
  const artifact = await TemporaryHandle.create('actions-mtx-manager-');

  try {
    const { packageName, version } = await readPackageMetadata();

    await $({
      cwd: import.meta.dirname
    })`vp run build`;

    await $({
      cwd: import.meta.dirname
    })`vp pm pack --pack-destination ${artifact.path}`;

    const files = await readdir(artifact.path);
    const fileName = files.find(file => file.endsWith('.tgz'));

    if (!fileName) {
      throw new Error('Expected package tarball to be created');
    }

    return {
      payload: {
        fileName,
        packageName,
        version
      },
      artifacts: {
        package: artifact.path
      }
    };
  } catch (error) {
    await artifact.dispose();
    throw error;
  }
});
