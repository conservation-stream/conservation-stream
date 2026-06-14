import { kebabCase } from 'change-case';
import { github, owner, repo } from '../auth/github.ts';
import { ProcessingError } from '../types/errors.ts';
import { getShaForExistingFile } from '../utils/github.ts';
import type { CommandContext } from './publish.ts';

export const updateCommand = async ({ document, collection }: CommandContext) => {
  const filename = `${kebabCase(document.data.title)}-${document.data.urlId}`;
  const markdown = document.data.text;
  const folder = kebabCase(collection.data.name);
  const branch = `kb-${document.data.urlId}`;

  const existingSha = await getShaForExistingFile(`apps/site/docs/${folder}/${filename}.md`, branch);
  if (!existingSha) return new ProcessingError(`File does not exist`);

  await github.rest.repos.createOrUpdateFileContents({
    owner,
    repo,
    path: `apps/site/src/kb/${folder}/${filename}.md`,
    message: `kb: ${document.data.title}`,
    content: Buffer.from(markdown).toString('base64'),
    branch: branch,
    sha: existingSha
  });
};
