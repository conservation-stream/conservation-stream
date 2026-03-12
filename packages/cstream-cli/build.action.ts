import { build } from '@conservation-stream/internal-actions';
import { $ } from 'zx';

export interface Payload {
  arch: string;
  image: string;
  sourceTag: string;
}

type Artifacts = never;

const BUILDER_NAME = 'cstream-cli-build';

export const build_matrix = {
  include: [
    {
      matrix_key: 'amd64',
      matrix_values_json: JSON.stringify({
        arch: 'amd64',
        runner: 'ubuntu-latest'
      }),
      runner: 'ubuntu-latest'
    },
    {
      matrix_key: 'arm64',
      matrix_values_json: JSON.stringify({
        arch: 'arm64',
        runner: 'ubuntu-24.04-arm'
      }),
      runner: 'ubuntu-24.04-arm'
    }
  ]
} as const;

const getImageName = (repository: string) => {
  const [owner] = repository.toLowerCase().split('/');
  if (!owner) {
    throw new Error(`Invalid repository name: ${repository}`);
  }

  return `ghcr.io/${owner}/mediamtx-cstream`;
};

const getSourceTag = (sha: string, arch: string) => `build-${sha.slice(0, 12)}-${arch}`;

const ensureArch = (arch: string | undefined) => {
  if (!arch) {
    throw new Error('matrix.arch is required');
  }

  const supportedArchitectures = build_matrix.include.map(entry => JSON.parse(entry.matrix_values_json).arch as string);

  if (!supportedArchitectures.includes(arch)) {
    throw new Error(`Unsupported architecture: ${arch}`);
  }

  return arch;
};

const loginToRegistry = async (actor: string, token: string) => {
  await $({
    input: `${token}\n`
  })`docker login ghcr.io --username ${actor} --password-stdin`;
};

const ensureBuilder = async () => {
  await $`docker run --privileged --rm tonistiigi/binfmt --install amd64,arm64`;

  const inspect = await $({ nothrow: true })`docker buildx inspect ${BUILDER_NAME}`;
  if (inspect.exitCode !== 0) {
    await $`docker buildx create --name ${BUILDER_NAME} --driver docker-container --use`;
  } else {
    await $`docker buildx use ${BUILDER_NAME}`;
  }

  await $`docker buildx inspect --bootstrap`;
};

await build<Payload, Artifacts>(async env => {
  const githubToken = process.env.GITHUB_TOKEN;
  if (!githubToken) {
    throw new Error('GITHUB_TOKEN is required to publish the container image');
  }

  const arch = ensureArch(env.matrix.arch);
  const image = getImageName(env.GITHUB_REPOSITORY);
  const sourceTag = getSourceTag(env.GITHUB_SHA, arch);

  await loginToRegistry(env.GITHUB_ACTOR, githubToken);
  await ensureBuilder();

  console.log(`Publishing ${image}:${sourceTag}`);
  console.log(`Architecture: ${arch}`);

  await $({
    cwd: env.GITHUB_WORKSPACE
  })`docker buildx build --platform ${`linux/${arch}`} --file ${`${import.meta.dirname}/Dockerfile`} --label ${`org.opencontainers.image.source=https://github.com/${env.GITHUB_REPOSITORY}`} --label ${`org.opencontainers.image.revision=${env.GITHUB_SHA}`} --tag ${`${image}:${sourceTag}`} --push .`;

  return {
    payload: {
      arch,
      image,
      sourceTag
    }
  };
});
