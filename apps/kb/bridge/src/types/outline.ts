import { z } from 'zod';

export const OutlineWebhookEvent = z.object({
  id: z.string().describe('The ID of the event. Ensure this is only processed once.'),
  actorId: z.string().describe('The ID of the actor (user, app) that triggered the event.'),
  webhookSubscriptionId: z.string(),
  createdAt: z.coerce.date(),
  event: z.string().describe('The type of event that occurred.'),
  payload: z.unknown()
});

export const OutlineCreatedBy = z.object({
  id: z.string(),
  name: z.string(),
  role: z.string(),
  isSuspended: z.boolean(),
  createdAt: z.coerce.date(),
  updatedAt: z.coerce.date()
});

export const OutlineEventCommentsCreatePayload = z.object({
  id: z.string(),
  model: z.object({
    id: z.string(),
    data: z.unknown(),
    documentId: z.string(),
    parentCommentId: z.string().nullable(),
    createdById: z.string(),
    createdBy: OutlineCreatedBy,
    createdAt: z.coerce.date(),
    updatedAt: z.coerce.date()
  })
});

export type OutlineEventCommentsCreatePayload = z.infer<typeof OutlineEventCommentsCreatePayload>;

export const ParagraphContentBlock = z.object({
  type: z.literal('paragraph'),
  content: z.array(
    z.object({
      type: z.literal('text'),
      text: z.string()
    })
  )
});

export const ContentBlock = z.discriminatedUnion('type', [ParagraphContentBlock]);

export const CommentDocument = z.object({
  type: z.literal('doc'),
  content: z.array(ContentBlock)
});

export const ValidCommands = z.union([z.literal('/publish'), z.literal('/update')]);
export type ValidCommands = z.infer<typeof ValidCommands>;

export interface Actor {
  id: string;
  name: string;
}

export interface OutlineDocument {
  data: {
    id: string;
    urlId: string;
    collectionId: string;
    title: string;
    text: string;
    collaborators?: Actor[];
    createdBy: Actor;
  };
}

export interface OutlineCollection {
  data: {
    id: string;
    name: string;
  };
}

export interface OutlineComment {
  data: {
    id: string;
  };
}

export interface OutlineDocumentExport {
  data: string;
}
