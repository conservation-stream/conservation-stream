import { createAppAuth } from '@octokit/auth-app';
import { Octokit } from 'octokit';
import { tri } from '../utils/tri.ts';

const private_key = `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEA21vPLr8Hq3twFZEfmsT540cXXVFaLW5Fmpq6gzW8AaRyyf95
jkuS3pIZZ/oZDeICeDE3U3HXO5uBRFiiOhiGtMvclN4F/HHMOOSOYXUBxcyMd+0A
7nbCzdeCYqYr3E61kF74G6tISmRMzc7Akj+JqFPXQ9wugO7QFBalmDBe8hV8PTt0
vSMonzsWkUYFXAeDInZZlsVehMPznSCnwk67VRyWB/evG1e+jHs9YkHi7XnifBtV
nH7RI5gq3ToRLLe8+CGy4EOM/AMK/Pxttz7SMpFL2XsqcrorzSmK2QRiqFjH8TTH
4UQo97jeohllWg6lSyp63iT223p8IxSC/6/OOQIDAQABAoIBAFfSM+plNK7k5oTb
6ylNfzYM+j+0GERkB53UIKPzWWfW7NYOYB4mB5DwMRe9u1YhjBeOrLXNt/v3UBOK
4LgdpaCwlYlMMl1VOgv1BUPjUmhcckh5LIxMI8XBaEQSfzDemjZIr1B0jKar7Uvg
YJysr3IizuYuOrgH5GqGfpmlk/2a+reDfvcm7F6VkosVsgLH7bMECZSsBdA/atRX
Nwqv6xmHdzroz3F8tLbaCqEaKtJaSIP5TbZUOK5AsoUXKTlxqSsNDSyvrKciF+ZU
pmKOdYyQy/038QKD2qT7nBIUWSAXoIHKJZ4gznhb/B5nZNGvekn+E/l/eWrzhgEg
TGs4jhkCgYEA7tE4TdgMbNndb7Q0UO80rRbaikkqw/Qquh5mbaHOyRYlDxoW8T0Q
4d3MnFN7NWuqokCUK66YOtYb85CAu49lo09lCh3lr2ttEw0V7dAg+14UykUW9mjI
7c+SaArQlI4bWxmpurw4PjrpxCJdBXOo+dCiai2Zs+gbwqSfEEjmCe8CgYEA6yQu
LMgcsJgsNnBWKuhThi+nKTIc7lCe5H7sEi7zNzZ0E1OVM2VyVijKmOjLcJmMBI6j
djE+O5Q1eKwwIXLUrgjC+r0g80GRUG1yMOtNw4Vf38qj38s3ieQBZa6RDJm8KSKb
Hex94/lz/U1+LC2eg3EVUzdFlrgADexgW7ZKclcCgYArUPmEbQ5749xdOXNPxsNo
Lb//2xuNpUjmr0Lm2bV3FbQtFA9bPDdGsIM/S4kKfHfbrBjz/1wPN+yj9e7TlkPa
JjluZ1PUyIhlLzduBhUlYsAkm/l5QjJHqCGnC2cfutLNaE831pHg/7CM6aqzpXHd
tfDvj0vUrOH0IQXU31QSMwKBgBXPmlTfDwI2a0t1ahi6yhyVSP0iP9q/Ma3iNAWP
w1GoxGWSiDFnRI7HY9uBJHXCWGGH1ZO+B5bBLaCO4DwKCb5G48ccSfUmbNM4A7KT
8Pek5Hq+siqtD+7Dbnm/EodHr1NleVvyNs8xsVeam4x/gseQcrjwVI0hbifceCep
pggrAoGBAJvAm7SeOvmiRNrnUI8R69y28qTgyQNFO1tlS164yjSPJ9tRDuT6AWY1
eqkHuNOvg7CmU1qF0PjsdJnyZzrW8YGlDVpkHCeUUG095qU89VNfkz69ccAxi/yB
5GaXI/PuyKS7oun+1UXKcGsorKejxaCNTfMUJbK/v9XdR/6rHtoM
-----END RSA PRIVATE KEY-----
`;

const appAuthManager = createAppAuth({
  appId: '2756474',
  privateKey: private_key,
  installationId: '106868150'
});

const auth = await appAuthManager({ type: 'installation' });
export const github = new Octokit({ auth: auth.token });

export const owner = 'conservation-stream';
export const repo = 'conservation-stream';
export const main = 'main';

export const getBaseRef = async () => {
  const result = await tri(() =>
    github.rest.git.getRef({
      owner,
      repo,
      ref: `heads/main`
    })
  );
  if (result instanceof Error) {
    throw result;
  }
  return result;
};
