import { local } from '@pulumi/command';
import * as pulumi from '@pulumi/pulumi';

interface DatabaseArgs {
  organization: string;
  clusterSize: string;
  engine: string;
  replicas: number;
  region: string;
}

interface RoleCreationResponse {
  access_host_url: string;
  database_url: pulumi.Output<string>;
  id: string;
  name: string;
  password: pulumi.Output<string>;
  username: string;
}

interface DatabaseCreationResponse {
  created_at: string;
  html_url: string;
  kind: string;
  name: string;
  notes: string;
  region: {
    current_default: boolean;
    display_name: string;
    enabled: boolean;
    location: string;
    provider: string;
    slug: string;
  };
  state: string;
  updated_at: string;
}

export class Database extends pulumi.ComponentResource {
  public readonly organization: string;
  public readonly clusterSize: string;
  public readonly engine: string;
  public readonly replicas: number;
  public readonly region: string;

  public readonly result: pulumi.Output<DatabaseCreationResponse>;
  public readonly role: pulumi.Output<RoleCreationResponse>;
  constructor(name: string, args: DatabaseArgs, opts?: pulumi.ComponentResourceOptions) {
    super('custom:planetscale:Database', name, args, opts);

    this.organization = args.organization;
    this.clusterSize = args.clusterSize;
    this.engine = args.engine;
    this.replicas = args.replicas;
    this.region = args.region;

    const serviceTokenId = pulumi.secret(process.env.PLANETSCALE_SERVICE_TOKEN_ID);
    const serviceToken = pulumi.secret(process.env.PLANETSCALE_SERVICE_TOKEN);

    const version = new local.Command(
      'get-version',
      {
        create: `pscale version -f json`
      },
      { parent: this }
    );

    const create = new local.Command(
      'create-database',
      {
        create: pulumi.interpolate`pscale database create ${name} --cluster-size ${this.clusterSize} --engine ${this.engine} --replicas ${this.replicas} --wait -f json --service-token ${serviceToken} --service-token-id ${serviceTokenId} --debug --org ${this.organization}`,
        delete: pulumi.interpolate`pscale database delete ${name} --service-token ${serviceToken} --service-token-id ${serviceTokenId} --org ${this.organization}`
      },
      { parent: this, dependsOn: [version] }
    );

    const password = new local.Command(
      'get-password',
      {
        create: pulumi.interpolate`pscale role create ${name} main primary1 --inherited-roles postgres -f json --service-token ${serviceToken} --service-token-id ${serviceTokenId} --org ${this.organization}`
      },
      { parent: this, dependsOn: [create] }
    );

    this.result = create.stdout.apply(stdout => {
      try {
        return JSON.parse(stdout) as DatabaseCreationResponse;
      } catch (error) {
        throw new Error(`Failed to create database: ${error} & ${stdout}`);
      }
    });
    this.role = password.stdout.apply(stdout => {
      try {
        const result = JSON.parse(stdout) as RoleCreationResponse;
        return {
          access_host_url: result.access_host_url,
          database_url: pulumi.secret(result.database_url),
          id: result.id,
          name: result.name,
          password: pulumi.secret(result.password),
          username: result.username
        };
      } catch (error) {
        throw new Error(`Failed to create role: ${error} & ${stdout}`);
      }
    });

    this.registerOutputs({
      name: name,
      organization: this.organization,
      clusterSize: this.clusterSize,
      engine: this.engine,
      result: this.result,
      role: this.role
    });
  }
}
