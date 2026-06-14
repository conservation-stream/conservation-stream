import { Hono } from 'hono';
import { executeCommand } from '../../commands/index.ts';
import { ProcessingError } from '../../types/errors.ts';
import { CommentDocument, OutlineEventCommentsCreatePayload, OutlineWebhookEvent, ValidCommands, type OutlineCollection, type OutlineDocument } from '../../types/outline.ts';
import { outline, type OutlineRedirect } from '../../utils/outline.ts';

const outlineWebhook = new Hono();

const processCommentForCommand = async (comment: OutlineEventCommentsCreatePayload) => {
  const doc = CommentDocument.safeParse(comment.model.data);
  if (!doc.success) return new ProcessingError(`Comment is not a valid document`);

  const paragraph = doc.data.content.find(block => block.type === 'paragraph');
  if (!paragraph) return new ProcessingError(`Comment does not contain a valid paragraph`);

  const text = paragraph.content.find(block => block.type === 'text');
  if (!text) return new ProcessingError(`Comment does not contain a valid text block`);

  // Does it start with a slash command?
  if (!text.text.startsWith('/')) return new ProcessingError(`Comment does not start with a valid slash command`);

  const [first, ...rest] = text.text.trim().split(' ');
  const command = ValidCommands.safeParse(first);
  if (!command.success) return new ProcessingError(`Comment does not contain a valid slash command`);

  return {
    command: command.data,
    text: rest.join(' ')
  };
};

outlineWebhook.post('/', async c => {
  const body = await c.req.json();

  const event = OutlineWebhookEvent.safeParse(body);
  if (!event.success) return c.text('Invalid request', 400);

  if (event.data.event !== 'comments.create') return c.text('OK');

  const comment = OutlineEventCommentsCreatePayload.safeParse(event.data.payload);
  if (!comment.success) return c.text('Invalid request', 400);

  const result = await processCommentForCommand(comment.data);
  if (result instanceof ProcessingError) {
    console.log(`Processing error: ${result.message}`);
    return c.text('OK');
  }

  const document = await outline<OutlineDocument>('documents.info', { id: comment.data.model.documentId });
  if (document instanceof Error) {
    console.error(document);
    return c.text('OK');
  }

  const collection = await outline<OutlineCollection>('collections.info', { id: document.data.collectionId });
  if (collection instanceof Error) {
    console.error(collection);
    return c.text('OK');
  }

  await executeCommand(result.command, {
    documentId: comment.data.model.documentId,
    commentId: comment.data.id,
    text: result.text,
    actor: {
      id: comment.data.model.createdById,
      name: comment.data.model.createdBy.name
    },
    document,
    collection
  });

  return c.text('OK');
});

export { outlineWebhook };

export const getRealAttachmentURL = async (path: string) => {
  if (!path.startsWith('/api/attachments.redirect')) return new Error('Not a redirect path');
  const [, _] = path.split('?');
  const params = new URLSearchParams(_);
  const id = params.get('id');
  if (!id) return new Error('No id found');

  const redirect = await outline<OutlineRedirect>('attachments.redirect', { id });
  if (redirect instanceof Error) {
    console.error(redirect);
    throw new Error('Failed to redirect');
  }

  const resolved = new URL(redirect.url);
  resolved.search = '';

  return resolved;
};
