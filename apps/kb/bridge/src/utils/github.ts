import { github, owner, repo } from '../auth/github.ts';
import { tri } from './tri.ts';

export const getShaForExistingFile = async (path: string, branch: string) => {
  const file = await tri(() =>
    github.rest.repos.getContent({
      owner,
      repo,
      path,
      ref: `heads/${branch}`
    })
  );
  if (file instanceof Error) {
    return undefined;
  }
  if (Array.isArray(file.data)) return undefined;
  if (file.data.type !== 'file') return undefined;
  return file.data.sha;
};
