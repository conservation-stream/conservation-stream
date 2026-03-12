import * as cloudflare from '@pulumi/cloudflare';
import * as pulumi from '@pulumi/pulumi';
import * as random from '@pulumi/random';

type PartialBy<T, K extends keyof T> = Omit<T, K> & Partial<Pick<T, K>>;

type RequiredBy<T, K extends keyof T> = Omit<T, K> & Required<Pick<T, K>>;

type Ingress = cloudflare.types.input.ZeroTrustTunnelCloudflaredConfigConfigIngress;

interface CoreCloudflareTunnelArgs {
  accountId: string;
  zoneId: pulumi.Output<string>;
}

interface CloudflareTunnelArgsWithTopLevelHostname extends CoreCloudflareTunnelArgs {
  hostname: string;
  ingress: PartialBy<Ingress, 'hostname'>[];
}

interface CloudflareTunnelArgsWithIngressHostname extends CoreCloudflareTunnelArgs {
  hostname?: never;
  ingress: RequiredBy<Ingress, 'hostname'>[];
}

export type CloudflareTunnelArgs = CloudflareTunnelArgsWithTopLevelHostname | CloudflareTunnelArgsWithIngressHostname;

export class CloudflareTunnel extends pulumi.ComponentResource {
  public readonly token: pulumi.Output<string>;
  public readonly tunnelId: pulumi.Output<string>;
  constructor(name: string, args: CloudflareTunnelArgs, opts?: pulumi.ComponentResourceOptions) {
    super('custom:cloudflare:CloudflareTunnel', name, args, opts);

    const secret = new random.RandomBytes(`${name}-secret`, { length: 32 }, { parent: this });

    const tunnel = new cloudflare.ZeroTrustTunnelCloudflared(
      `${name}-tunnel`,
      {
        accountId: args.accountId,
        name,
        configSrc: 'cloudflare',
        tunnelSecret: secret.base64
      },
      { parent: this }
    );

    const hostnames = new Set<string>();
    if (args.hostname) hostnames.add(args.hostname);
    args.ingress.forEach((ingress: Ingress) => {
      if (ingress.hostname) {
        if (typeof ingress.hostname !== 'string') throw new Error('hostname must be a string');
        hostnames.add(ingress.hostname);
      }
    });

    const config = new cloudflare.ZeroTrustTunnelCloudflaredConfig(
      `${name}-config`,
      {
        accountId: args.accountId,
        tunnelId: tunnel.id,
        config: {
          ingresses: [
            ...args.ingress.map((ingress: Ingress) => ({
              ...ingress,
              hostname: ingress.hostname ?? args.hostname
            })),
            { service: 'http_status:404' }
          ]
        }
      },
      { parent: this }
    );

    for (const hostname of hostnames) {
      const record = new cloudflare.DnsRecord(
        `${name}-${hostname}-cname`,
        {
          zoneId: args.zoneId,
          type: 'CNAME',
          name: hostname,
          content: pulumi.interpolate`${tunnel.id}.cfargotunnel.com`,
          proxied: true,
          ttl: 1,
          comment: `Tunnel for ${name}`
        },
        { parent: this, dependsOn: [config] }
      );
    }

    const output = cloudflare.getZeroTrustTunnelCloudflaredTokenOutput({
      accountId: args.accountId,
      tunnelId: tunnel.id
    });

    this.token = output.apply(token => token.token);
    this.tunnelId = pulumi.output(tunnel.id);
  }
}

export const composeConfigFromTunnel = (tunnel: CloudflareTunnel) => {
  return {
    cf_tunnel: {
      image: 'cloudflare/cloudflared:latest',
      restart: 'unless-stopped',
      command: 'tunnel run',
      environment: {
        TUNNEL_TOKEN: tunnel.token
      }
    }
  };
};
