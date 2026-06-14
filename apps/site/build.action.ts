import { build } from "@conservation-stream/internal-actions";
import { tmpdir } from "node:os";
import path from "node:path";
import { $, fs } from "zx";

export type Artifacts = "api" | "ui";

export class TemporaryHandle {
  public readonly path: string;
  constructor() {
    const id = crypto.randomUUID();
    this.path = path.join(tmpdir(), `actions/${id}`);
  }
  [Symbol.dispose]() {
    fs.rm(this.path, { recursive: true });
  }
}

await build<unknown, Artifacts>(async () => {
  const api = new TemporaryHandle();
  const ui = new TemporaryHandle();

  await $`pnpm --filter @conservation-stream/site build`;
  await $`cp -r ${path.join(import.meta.dirname, "site", "build", "client")} ${ui.path}`;

  await fs.mkdir(api.path, { recursive: true });
  await $`pnpm --filter @conservation-stream/site-api deploy ${api.path} --legacy`;

  return {
    payload: {},
    artifacts: {
      api: api.path,
      ui: ui.path,
    },
  };
});
