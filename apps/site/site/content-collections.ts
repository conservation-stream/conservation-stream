import { defineCollection, defineConfig } from '@content-collections/core';
import { z } from 'zod';
import { buildTableOfContents } from './app/lib/toc';

const posts = defineCollection({
  name: 'posts',
  directory: '../docs',
  include: '**/*.md',
  schema: z.object({
    title: z.string().optional()
  }),
  transform: content => {
    const toc = buildTableOfContents(content.content);
    return { ...content, toc };
  }
});

export default defineConfig({
  content: [posts]
});
