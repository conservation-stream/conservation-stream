import { kebabCase } from 'change-case';
import type { Image } from 'mdast';
import remarkParse from 'remark-parse';
import remarkStringify from 'remark-stringify';
import { unified } from 'unified';
import { getBaseRef, github, main, owner, repo } from '../auth/github.ts';
import { ProcessingError } from '../types/errors.ts';
import type { Actor, OutlineCollection, OutlineComment, OutlineDocument } from '../types/outline.ts';
import { getShaForExistingFile } from '../utils/github.ts';
import { mux } from '../utils/mux.ts';
import { outline } from '../utils/outline.ts';
import { poll } from '../utils/poll.ts';
import { tri } from '../utils/tri.ts';
import { walk } from '../utils/walk.ts';
import { getRealAttachmentURL } from '../webhook/outline/route.ts';

export interface CommandContext {
  documentId: string;
  commentId: string;
  text: string;
  actor: Actor;
  document: OutlineDocument;
  collection: OutlineCollection;
}

export const publishCommand = async ({ document, commentId, collection, text, actor }: CommandContext) => {
  const filename = `${kebabCase(document.data.title)}-${document.data.urlId}`;
  const markdown = document.data.text;
  const folder = kebabCase(collection.data.name);
  const branch = `kb-${document.data.urlId}`;

  const report = await outline<OutlineComment>('comments.create', {
    documentId: document.data.id,
    parentCommentId: commentId,
    text: `✨ Publishing...`
  });

  if (report instanceof Error) {
    return report;
  }

  const result = await tri(() =>
    github.rest.git.getRef({
      owner,
      repo,
      ref: `heads/${branch}`
    })
  );

  if (result instanceof Error) {
    const base = await getBaseRef();
    await github.rest.git.createRef({
      owner,
      repo,
      ref: `refs/heads/${branch}`,
      sha: base.data.object.sha
    });
  }

  const sha = await getShaForExistingFile(`apps/site/docs/${folder}/${filename}.md`, branch);

  const file = unified().use(remarkParse).parse(markdown);

  await walk(file, match => [
    match({
      if: (node): node is Image => node.type === 'image' || (node.type === 'link' && node.url.startsWith('/api/attachments.redirect')),
      update: async node => {
        const url = await getRealAttachmentURL(node.url);
        if (url instanceof Error) {
          console.error(url);
          return;
        }

        if (node.title?.trim().startsWith('=')) {
          const title = node.title.trim().replace('=', '');
          const [width, height] = title.split('x').map(Number);
          url.searchParams.set('width', width.toString());
          url.searchParams.set('height', height.toString());
          url.searchParams.set('aspect-ratio', (height / width).toFixed(3));

          node.type = 'image';
          node.url = url.toString();
        }

        if (url.pathname.endsWith('.mp4')) {
          const asset = await mux.video.assets.create({
            inputs: [{ type: 'video', url: url.toString() }],
            video_quality: 'basic',
            playback_policies: ['public']
          });

          if (!asset.playback_ids) throw new ProcessingError('Failed to create asset');
          const publicPlayback = asset.playback_ids.find(playback => playback.policy === 'public');
          if (!publicPlayback) throw new ProcessingError('Failed to create asset');

          const playback = await poll(
            () => mux.video.assets.retrieve(asset.id),
            a => a.status === 'ready'
          );

          const muxUrl = new URL(`https://stream.mux.com/${publicPlayback.id}.m3u8`);
          const aspect = playback.aspect_ratio;
          if (!aspect) throw new ProcessingError('Failed to get aspect ratio');
          const [width, height] = aspect.split(':').map(Number);
          muxUrl.searchParams.set('width', width.toString());
          muxUrl.searchParams.set('height', height.toString());
          muxUrl.searchParams.set('aspect-ratio', (height / width).toFixed(3));
          console.log(playback, muxUrl);

          node.title = undefined;
          node.type = 'image';
          node.url = muxUrl.toString();
        }
      }
    })
  ]);

  const output = await unified().use(remarkStringify).stringify(file);
  console.log(output);

  await github.rest.repos.createOrUpdateFileContents({
    owner,
    repo,
    path: `apps/site/docs/${folder}/${filename}.md`,
    message: `kb: ${document.data.title}`,
    content: Buffer.from(output, 'utf8').toString('base64'),
    branch: branch,
    sha: sha
  });

  await outline<OutlineComment>('comments.update', {
    id: report.data.id,
    data: comment(`✨ Creating pull request...`)
  });

  const pr = await tri(() =>
    github.rest.pulls.create({
      owner,
      repo,
      title: `kb: ${text}`,
      body: `${text}\n<i>This document has been authored by ${[document.data.createdBy, ...(document.data.collaborators || [])].map(c => c.name).join(', ')}</i>\n<sub>This is an automated pull request requested by ${actor.name}</sub>`,
      base: main,
      head: branch,
      maintainer_can_modify: true
    })
  );

  if (pr instanceof Error) {
    console.error(pr);
    return new ProcessingError(`Failed to create pull request`);
  }

  const link = `https://github.com/${owner}/${repo}/pull/${pr.data.number}`;
  await outline<OutlineComment>('comments.update', {
    id: report.data.id,
    data: comment(`✨ Pull request is live`, { link })
  });
};

interface CommentOptions {
  link?: string;
}

const comment = (text: string, options: CommentOptions = {}) => {
  const marks = [];

  if (options.link) {
    marks.push({ type: 'link', attrs: { href: options.link, title: null } });
  }

  return {
    type: 'doc',
    content: [
      {
        type: 'paragraph',
        content: [{ type: 'text', text, marks }]
      }
    ]
  };
};
