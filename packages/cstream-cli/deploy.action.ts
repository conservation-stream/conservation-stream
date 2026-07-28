import { deploy } from '@conservation-stream/internal-actions';
import { execFile } from 'node:child_process';
import type { Payload } from './build.action.ts';
import { promisify } from 'node:util';
import { $ } from 'zx';

const execFileAsync = promisify(execFile);

type Artifacts = never;

const getBuilds = (builds: Payload[]) => {
  const [first, ...rest] = builds;
  if (!first) {
    throw new Error('No Docker publish configuration found');
  }

  for (const build of rest) {
    if (build.image !== first.image) {
      throw new Error('Mismatched Docker image names across build jobs');
    }
  }

  const byArch = new Map<string, Payload>();
  for (const build of builds) {
    if (byArch.has(build.arch)) {
      throw new Error(`Duplicate Docker build for architecture: ${build.arch}`);
    }
    byArch.set(build.arch, build);
  }

  return {
    image: first.image,
    builds: [...byArch.values()].sort((a, b) => a.arch.localeCompare(b.arch))
  };
};

const getDateTag = (date: Date = new Date()) => {
  const year = date.getUTCFullYear().toString().padStart(4, '0');
  const month = (date.getUTCMonth() + 1).toString().padStart(2, '0');
  const day = date.getUTCDate().toString().padStart(2, '0');
  return `${year}${month}${day}`;
};

const ensureRelease = async (repo: string, tag: string, image: string, dateTag: string) => {
  const notes = [`Published container image \`${image}\`.`, '', `- \`${image}:${dateTag}\``, `- \`${image}:latest\``].join('\n');

  try {
    await execFileAsync('gh', ['release', 'view', tag, '--repo', repo], {
      env: process.env
    });
    console.log(`GitHub release ${tag} already exists`);
  } catch {
    try {
      await execFileAsync('gh', ['release', 'create', tag, '--repo', repo, '--title', tag, '--notes', notes], {
        env: process.env
      });
    } catch (error) {
      const stderr = error instanceof Error && 'stderr' in error ? String(error.stderr) : '';
      if (stderr.includes('Resource not accessible by integration')) {
        throw new Error('Creating a GitHub release requires `contents: write` permission on GITHUB_TOKEN');
      }

      throw error;
    }
  }
};

const loginToRegistry = async (actor: string, token: string) => {
  await $({
    input: `${token}\n`
  })`docker login ghcr.io --username ${actor} --password-stdin`;
};

const getTargetTagArgs = (image: string, dateTag: string) => ['--tag', `${image}:${dateTag}`, '--tag', `${image}:latest`];

const getSourceImages = (image: string, builds: Payload[]) => builds.map(build => `${image}:${build.sourceTag}`);

await deploy<Payload, Artifacts>(async env => {
  const release = getBuilds(env.build);
  const githubToken = process.env.GITHUB_TOKEN;

  if (!githubToken) {
    throw new Error('GITHUB_TOKEN is required to publish the container image');
  }

  await loginToRegistry(env.GITHUB_ACTOR, githubToken);

  const dateTag = getDateTag();

  console.log(`Creating multi-arch tags for ${release.image}`);
  console.log(`Architectures: ${release.builds.map(build => build.arch).join(', ')}`);
  console.log(`Tags: ${dateTag}, latest`);

  await $`docker buildx imagetools create ${getTargetTagArgs(release.image, dateTag)} ${getSourceImages(release.image, release.builds)}`;

  await ensureRelease(env.GITHUB_REPOSITORY, env.GITHUB_REF_NAME, release.image, dateTag);
});
