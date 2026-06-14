import Slugger from 'github-slugger';
import type { Heading } from 'mdast';
import { toString } from 'mdast-util-to-string';
import remarkParse from 'remark-parse';
import { unified } from 'unified';
import { visit } from 'unist-util-visit';
import z from 'zod';

const Section = z.object({
  title: z.string(),
  url: z.string(),
  get sections() {
    return z.array(Section).optional();
  }
});

export type Section = z.infer<typeof Section>;

export const buildTableOfContents = (content: string): Section[] => {
  if (!content.trim()) return [];

  const slugger = new Slugger();
  const root = unified().use(remarkParse).parse(content);
  const toc: Section[] = [];
  const stack: Array<{ depth: number; section: Section }> = [];

  visit(root, 'heading', (heading: Heading) => {
    const title = toString(heading).trim();
    if (!title) return;

    const id = slugger.slug(title);
    const entry: Section = { title, url: `#${id}` };

    while (stack.length && heading.depth <= stack[stack.length - 1].depth) {
      stack.pop();
    }

    if (stack.length === 0) {
      toc.push(entry);
    } else {
      const parent = stack[stack.length - 1].section;
      parent.sections ||= [];
      parent.sections.push(entry);
    }

    stack.push({ depth: heading.depth, section: entry });
  });

  return toc;
};
