import * as pulumi from '@pulumi/pulumi';
import dedent from 'dedent';
import { writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import * as yaml from 'yaml';
import type { CloudInitService } from '../init/CloudInit';
import type { ComposeSpecification } from './compose.types';

/**
 * Recursively wraps all leaf values with pulumi.Input<T>, allowing
 * nested Output values anywhere in the configuration.
 */
type DeepInput<T> = T extends (infer U)[] ? DeepInput<U>[] : T extends object ? { [K in keyof T]?: DeepInput<T[K]> } : pulumi.Input<T>;

interface ComposeArgs {
  name: string;
  dir: string;
  config: DeepInput<ComposeSpecification>;
}

export class Compose extends pulumi.ComponentResource {
  public readonly name: string;
  public readonly dir: string;
  public readonly config: pulumi.Output<pulumi.UnwrappedObject<ComposeSpecification>>;
  public readonly rendered: pulumi.Output<string>;
  public readonly path: string;
  constructor(name: string, args: ComposeArgs, opts?: pulumi.ComponentResourceOptions) {
    super('custom:compose:Compose', name, args, opts);
    this.name = args.name;
    this.dir = args.dir;
    // pulumi.output() deeply resolves all nested Input values
    this.config = pulumi.output(args.config as pulumi.Input<ComposeSpecification>);
    this.path = `${tmpdir()}/${this.name}-docker-compose.yml`;
    writeFileSync(this.path, '');

    this.rendered = this.config.apply(config => {
      const rendered = yaml.stringify(config);
      writeFileSync(this.path, rendered);
      return rendered;
    });
  }
}

export const cloudConfigFromCompose = (compose: Compose): Record<string, CloudInitService> => {
  return {
    [compose.name]: {
      writeFiles: [
        {
          path: `/etc/systemd/system/${compose.name}.service`,
          permissions: '0644',
          content: dedent`
            [Unit]
            Description=${compose.name} (docker compose)
            After=docker.service network-online.target
            Requires=docker.service

            [Service]
            Type=oneshot
            RemainAfterExit=yes
            WorkingDirectory=${compose.dir}
            ExecStart=/usr/bin/docker compose -f ${compose.dir}/docker-compose.yml up -d --remove-orphans
            ExecStop=/usr/bin/docker compose -f ${compose.dir}/docker-compose.yml down
            TimeoutStartSec=0

            [Install]
            WantedBy=multi-user.target
          `
        }
      ],
      runcmd: [
        dedent`
          set -eu
          mkdir -p ${compose.dir}
          # Enable service
          systemctl daemon-reload
          systemctl enable --now ${compose.name}.service
        `
      ]
    }
  };
};
