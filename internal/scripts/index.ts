#!/usr/bin/env node

import {
  before,
  writeOutput,
  type MatrixConfig,
} from "@conservation-stream/internal-actions";
import { execSync } from "node:child_process";
import { existsSync } from "node:fs";
import path from "node:path";
import { pathToFileURL } from "node:url";

interface WorkflowMatrixEntry {
  matrix_key: string;
  matrix_values_json: string;
  runner?: string;
}

interface WorkflowMatrix {
  include: WorkflowMatrixEntry[];
}

/** Walk up from dir looking for build.action.ts + deploy.action.ts */
const findActionDir = (dir: string, root: string): string | null => {
  if (
    existsSync(path.join(dir, "build.action.ts")) &&
    existsSync(path.join(dir, "deploy.action.ts"))
  ) {
    return dir;
  }
  const parent = path.dirname(dir);
  return parent === root ? null : findActionDir(parent, root);
};

/** Import build.action.ts and extract explicit build_matrix or a MatrixConfig export. */
const getBuildMatrix = async (filePath: string): Promise<WorkflowMatrix | null> => {
  process.env.MATRIX_ONLY = "1";
  try {
    const mod = await import(pathToFileURL(filePath).href);
    if (mod.build_matrix) {
      return mod.build_matrix as WorkflowMatrix;
    }
    if (mod.matrix) {
      return expandMatrix(mod.matrix as MatrixConfig);
    }
    return null;
  } catch {
    return null;
  } finally {
    delete process.env.MATRIX_ONLY;
  }
};

/** Cartesian product of matrix values → GH Actions format */
const expandMatrix = (matrix: MatrixConfig): WorkflowMatrix => {
  let combos: Record<string, string>[] = [{}];
  for (const key of Object.keys(matrix)) {
    combos = combos.flatMap((combo) =>
      matrix[key].map((v) => ({ ...combo, [key]: v })),
    );
  }
  return {
    include: combos.map((c) => ({
      matrix_key: Object.values(c).join("-"),
      matrix_values_json: JSON.stringify(c),
    })),
  };
};

await before(async (env) => {
  const baseSha = env.event.before;
  if (!baseSha) {
    throw new Error("Missing event.before for determine-changes");
  }

  const output = execSync(
    `pnpm -r --stream --filter "...[${baseSha}]" exec -- pwd`,
  );
  const changed = output.toString("utf-8").split("\n").filter(Boolean);

  const packages = new Map<string, string>();
  for (const line of changed) {
    if (line === env.GITHUB_WORKSPACE) continue;
    const dir = findActionDir(line, env.GITHUB_WORKSPACE);
    if (dir) packages.set(dir, path.basename(dir));
  }

  const include = await Promise.all(
    [...packages].map(async ([dir, name]) => {
      const build_matrix = await getBuildMatrix(path.join(dir, "build.action.ts"));

      if (build_matrix) {
        console.log(`${name}: ${build_matrix.include.length} matrix jobs`);
        return { dir, name, build_matrix };
      }

      return { dir, name };
    }),
  );

  await writeOutput(env.GITHUB_OUTPUT, {
    count: include.length,
    matrix: { include },
  });
});
