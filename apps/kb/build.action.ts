import { build } from '@conservation-stream/internal-actions';

export interface Payload {
  arch: string;
  digest: string;
  timestamp: Date;
}

type Artifacts = never; // No artifacts for this build

await build<Payload, Artifacts>(async env => {
  // env.matrix contains the current matrix values, e.g. { arch: "amd64" }
  console.log(`Building for architecture: ${env.matrix.arch}`);
  // Simulate building for this architecture
  const digest = `sha256:${crypto.randomUUID().replace(/-/g, '')}`;

  // Docker build the bridge on a single architecture

  return {
    payload: {
      arch: env.matrix.arch,
      digest,
      timestamp: new Date()
    }
  };
});
