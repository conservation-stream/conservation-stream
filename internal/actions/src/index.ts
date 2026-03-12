import fs from 'node:fs/promises';
import path from 'node:path';
import { env } from 'node:process';
import superjson from 'superjson';
import { z } from 'zod';

export const CoreGithubActionEnvironment = z
  .object({
    // General / CI
    GITHUB_ACTIONS: z.string(),

    // Current step/action
    GITHUB_ACTION: z.string(),
    GITHUB_ACTION_PATH: z.string().optional(), // composite actions only
    GITHUB_ACTION_REPOSITORY: z.string(),

    // Actor
    GITHUB_ACTOR: z.string(),
    GITHUB_ACTOR_ID: z.string().optional(),
    GITHUB_TRIGGERING_ACTOR: z.string().optional(),

    // API endpoints
    GITHUB_API_URL: z.string(),
    GITHUB_GRAPHQL_URL: z.string(),
    GITHUB_SERVER_URL: z.string(),

    // Event
    GITHUB_EVENT_NAME: z.string(),
    GITHUB_EVENT_PATH: z.string(),
    GITHUB_BASE_REF: z.string().optional(), // PR only
    GITHUB_HEAD_REF: z.string().optional(), // PR only

    // Job / run
    GITHUB_JOB: z.string(),
    GITHUB_RUN_ID: z.string(),
    GITHUB_RUN_NUMBER: z.string(),
    GITHUB_RUN_ATTEMPT: z.string(),
    GITHUB_RETENTION_DAYS: z.string(),
    GITHUB_WORKFLOW: z.string(),
    GITHUB_WORKFLOW_REF: z.string().optional(),
    GITHUB_WORKFLOW_SHA: z.string().optional(),

    // Repo / ref / commit
    GITHUB_REPOSITORY: z.string(),
    GITHUB_REPOSITORY_ID: z.string().optional(),
    GITHUB_REPOSITORY_OWNER: z.string(),
    GITHUB_REPOSITORY_OWNER_ID: z.string().optional(),
    GITHUB_REF: z.string(),
    GITHUB_REF_NAME: z.string(),
    GITHUB_REF_TYPE: z.string(),
    GITHUB_REF_PROTECTED: z.string().optional(),
    GITHUB_SHA: z.string(),

    // Filesystem + workflow command files
    GITHUB_WORKSPACE: z.string(),
    GITHUB_ENV: z.string(),
    GITHUB_OUTPUT: z.string(),
    GITHUB_PATH: z.string(),
    GITHUB_STEP_SUMMARY: z.string(),

    // Runner
    RUNNER_NAME: z.string().optional(),
    RUNNER_OS: z.string(),
    RUNNER_ARCH: z.string(),
    RUNNER_ENVIRONMENT: z.string().optional(),
    RUNNER_DEBUG: z.string().optional(), // only set when debugging is enabled
    RUNNER_TEMP: z.string(),
    RUNNER_TOOL_CACHE: z.string(),

    SECRETS: z.string().optional(),
    ACTIONS_RUNTIME_TOKEN: z.string().optional(),
    ACTIONS_CACHE_URL: z.string().optional()
  })
  .loose();

export type CoreGithubActionEnvironment = z.infer<typeof CoreGithubActionEnvironment>;

export type MatrixConfig = Record<string, readonly string[]>;

const parseMatrixEnv = (): Record<string, string> => {
  const json = env.MATRIX_VALUES_JSON;
  if (!json || json === '{}') return {};
  try {
    return JSON.parse(json) as Record<string, string>;
  } catch {
    return {};
  }
};

export const writeOutput = async (path: string, outputs: Record<string, string | number | boolean | object>) => {
  const output = Object.entries(outputs)
    .map(([key, value]) => {
      const stringValue = typeof value === 'object' ? JSON.stringify(value) : String(value);

      if (stringValue.includes('\n')) {
        let delimiter = 'EOF';
        while (stringValue.includes(delimiter)) {
          delimiter = `${delimiter}_`;
        }
        return `${key}<<${delimiter}\n${stringValue}\n${delimiter}`;
      }

      return `${key}=${stringValue}`;
    })
    .join('\n');
  await fs.writeFile(path, output, 'utf8');
};

export interface ArtifactConfig {
  path: string;
  include?: string[];
}

interface BuildResult<TPayload, TArtifacts extends string> {
  payload?: TPayload;
  artifacts?: Record<TArtifacts, string | ArtifactConfig>;
}

type BuildEnv = CoreGithubActionEnvironment & {
  event: { before: string };
  matrix: Record<string, string>;
};

type BeforeEnv = CoreGithubActionEnvironment & {
  event: { before?: string };
};

export const before = async (fn: (env: BeforeEnv) => Promise<void> | void) => {
  if (env.MATRIX_ONLY) return; // Allow importing for matrix extraction without running
  const parsed = CoreGithubActionEnvironment.parse(env);
  const event = await fs.readFile(parsed.GITHUB_EVENT_PATH, 'utf8');

  await fn({
    ...parsed,
    event: JSON.parse(event)
  });
};

export const build = async <TPayload = void, TArtifacts extends string = never>(fn: (env: BuildEnv) => Promise<void | BuildResult<TPayload, TArtifacts>> | void | BuildResult<TPayload, TArtifacts>) => {
  if (env.MATRIX_ONLY) return; // Allow importing for matrix extraction without running
  const parsed = CoreGithubActionEnvironment.parse(env);
  const event = await fs.readFile(parsed.GITHUB_EVENT_PATH, 'utf8');
  const matrix = parseMatrixEnv();

  const result = await fn({
    ...parsed,
    event: JSON.parse(event),
    matrix
  });

  if (!result) return;

  const artifactPaths: string[] = [];
  const artifactMap: Record<string, string> = {};
  const workspaceRoot = parsed.GITHUB_WORKSPACE;
  const ciName = env.CI_NAME ?? 'ci';
  const stagingRoot = path.join(workspaceRoot, '.artifacts', ciName);

  const ensureWorkspaceRelativePath = async (name: string, artifactPath: string) => {
    const absolutePath = path.isAbsolute(artifactPath) ? artifactPath : path.resolve(process.cwd(), artifactPath);

    if (absolutePath.startsWith(`${workspaceRoot}${path.sep}`) || absolutePath === workspaceRoot) {
      return path.relative(workspaceRoot, absolutePath);
    }

    await fs.mkdir(stagingRoot, { recursive: true });
    const stat = await fs.stat(absolutePath);

    if (stat.isDirectory()) {
      const targetDir = path.join(stagingRoot, name);
      await fs.rm(targetDir, { recursive: true, force: true });
      await fs.cp(absolutePath, targetDir, { recursive: true });
      return path.relative(workspaceRoot, targetDir);
    }

    const targetDir = path.join(stagingRoot, name);
    await fs.rm(targetDir, { recursive: true, force: true });
    await fs.mkdir(targetDir, { recursive: true });
    const targetFile = path.join(targetDir, path.basename(absolutePath));
    await fs.copyFile(absolutePath, targetFile);
    return path.relative(workspaceRoot, targetFile);
  };

  if (result.artifacts) {
    for (const [name, config] of Object.entries(result.artifacts)) {
      const artifactPath = typeof config === 'string' ? config : (config as ArtifactConfig).path;
      const relativePath = await ensureWorkspaceRelativePath(name, artifactPath);
      artifactPaths.push(relativePath);
      artifactMap[name] = relativePath;
    }
  }

  const metadata = {
    payload: result.payload,
    artifacts: artifactMap
  };

  // Write outputs for workflow to consume
  await writeOutput(parsed.GITHUB_OUTPUT, {
    metadata: superjson.stringify(metadata),
    artifact_paths: artifactPaths.join('\n')
  });
};

type DeployEnv<TPayload, TArtifacts extends string> = CoreGithubActionEnvironment & {
  event: { before: string };
  matrix: Record<string, string>;
  build: TPayload[];
  artifacts: Record<TArtifacts, string>;
};

/**
 * Deploy function - receives build outputs from artifacts.
 * For matrix builds, aggregates payloads from all matrix entries.
 *
 * @example
 * type Payload = { version: string; digest: string };
 * type Artifacts = "build" | "logs";
 * await deploy<Payload, Artifacts>(async (env) => {
 *   console.log(env.build.version);
 *   console.log(env.artifacts.build);
 * });
 */
export const deploy = async <TPayload = void, TArtifacts extends string = never>(fn: (env: DeployEnv<TPayload, TArtifacts>) => Promise<void> | void): Promise<void> => {
  if (env.MATRIX_ONLY) return; // Allow importing for matrix extraction without running

  const parsed = CoreGithubActionEnvironment.parse(env);
  const event = await fs.readFile(parsed.GITHUB_EVENT_PATH, 'utf8');
  const matrix = parseMatrixEnv();

  // Read artifacts directory
  const artifactsDir = env.ARTIFACTS_DIR;
  if (!artifactsDir) {
    throw new Error('ARTIFACTS_DIR environment variable is required');
  }

  const ciName = env.CI_NAME;
  if (!ciName) throw new Error('CI_NAME environment variable is required');

  // Scan for metadata artifacts
  const metadataFiles: string[] = [];
  try {
    const entries = await fs.readdir(artifactsDir, { withFileTypes: true });
    for (const entry of entries) {
      if (!entry.isDirectory()) continue;
      if (!entry.name.startsWith(ciName)) continue;
      if (entry.name.endsWith('-metadata')) {
        const metadataPath = `${artifactsDir}/${entry.name}/metadata.json`;
        try {
          await fs.access(metadataPath);
          metadataFiles.push(metadataPath);
        } catch {
          // Skip if metadata.json doesn't exist
        }
      }
    }
  } catch (error) {
    throw new Error(`Failed to read artifacts directory: ${error}`);
  }

  // Aggregate payloads and artifacts from all matrix entries
  const payloads: TPayload[] = [];
  const artifactPaths: Record<string, string> = {};

  for (const metadataPath of metadataFiles) {
    try {
      const metadataContent = (await fs.readFile(metadataPath, 'utf8')).trim();
      // Deserialize with superjson to restore Date objects, etc.
      const metadata = superjson.parse<{
        payload?: TPayload;
        artifacts?: Record<string, string>;
      }>(metadataContent);

      if (metadata.payload) {
        payloads.push(metadata.payload);
      }

      // Merge artifact paths (last one wins for same name)
      if (metadata.artifacts) {
        Object.assign(artifactPaths, metadata.artifacts);
      }
    } catch (error) {
      console.warn(`Failed to parse metadata file ${metadataPath}:`, error);
    }
  }

  const buildPayload = payloads;

  const artifacts: Record<string, string> = {};
  const artifactDirs = (await fs.readdir(artifactsDir, { withFileTypes: true }))
    .filter(entry => entry.isDirectory())
    .map(entry => entry.name)
    .filter(name => name.startsWith(ciName) && name.endsWith('-artifacts'))
    .map(name => `${artifactsDir}/${name}`);

  const resolveArtifactPath = async (relativePath: string): Promise<string> => {
    const normalizedPath = relativePath.startsWith('./') ? relativePath.slice(2) : relativePath;
    const baseCandidates = [normalizedPath, relativePath];
    const basenameCandidates = baseCandidates.map(candidate => path.basename(candidate));
    const candidates = [...baseCandidates, ...basenameCandidates];

    for (const dir of artifactDirs) {
      for (const candidate of candidates) {
        const fullPath = `${dir}/${candidate}`;
        try {
          await fs.access(fullPath);
          return fullPath;
        } catch {
          // Keep trying other candidates/directories
        }
      }
      // If the artifact path was a directory, upload-artifact downloads
      // the directory contents directly into the artifact root.
      try {
        const entries = await fs.readdir(dir);
        if (entries.length > 0) {
          return dir;
        }
      } catch {
        // Ignore and continue
      }
    }

    return relativePath;
  };

  for (const [name, relativePath] of Object.entries(artifactPaths)) {
    artifacts[name] = await resolveArtifactPath(relativePath);
  }

  await fn({
    ...parsed,
    event: JSON.parse(event),
    matrix,
    build: buildPayload,
    artifacts: artifacts as Record<TArtifacts, string>
  });
};
