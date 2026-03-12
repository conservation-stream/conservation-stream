import * as cloudinit from '@pulumi/cloudinit';
import * as pulumi from '@pulumi/pulumi';
import dedent from 'dedent';
import * as yaml from 'yaml';

interface Settings {
  docker: boolean;
}

interface WriteFile {
  path: string;
  permissions: string;
  content: string;
}

export interface CloudInitService {
  writeFiles: WriteFile[];
  runcmd: string[];
}

export interface CloudInitArgs {
  settings: Settings;
  services: Record<string, CloudInitService>;
}

export class CloudInit extends pulumi.ComponentResource {
  private readonly writeFiles: WriteFile[];
  private readonly runcmd: string[];
  public readonly rendered: pulumi.Output<string>;

  constructor(name: string, args: CloudInitArgs, opts?: pulumi.ComponentResourceOptions) {
    super('custom:init:CloudInit', name, args, opts);
    this.writeFiles = [];
    this.runcmd = [];

    if (args.settings.docker) {
      this.runcmd.push(dedent`
        set -eu
        # Install Docker Engine + Compose plugin (official apt repo)
        apt-get update
        apt-get install -y ca-certificates curl
        install -m 0755 -d /etc/apt/keyrings
        curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
        chmod a+r /etc/apt/keyrings/docker.asc

        ARCH="$(dpkg --print-architecture)"
        CODENAME="$(. /etc/os-release && echo "$VERSION_CODENAME")"
        echo "deb [arch=$ARCH signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $CODENAME stable" \
          > /etc/apt/sources.list.d/docker.list

        apt-get update
        apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
      `);
    }

    for (const service of Object.values(args.services)) {
      this.writeFiles.push(...service.writeFiles);
      this.runcmd.push(...service.runcmd);
    }

    const content = yaml.stringify({
      package_update: true,
      write_files: this.writeFiles,
      runcmd: this.runcmd
    });
    const header = `#cloud-config`;
    const userData = new cloudinit.Config(
      `${name}-userdata`,
      {
        gzip: false,
        base64Encode: false, // DO accepts plain user-data
        parts: [
          {
            contentType: 'text/cloud-config',
            content: `${header}\n${content}`
          }
        ]
      },
      { parent: this }
    );
    this.rendered = userData.rendered;
  }
}
