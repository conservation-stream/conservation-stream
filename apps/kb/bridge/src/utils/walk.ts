import type { Nodes, RootContent } from 'mdast';

export interface Rule<N extends RootContent = RootContent> {
  if: (node: RootContent) => node is N;
  update: (node: N) => void | Promise<void>;
}

export async function walk(root: Nodes, build: (match: <N extends RootContent>(rule: Rule<N>) => Rule) => Rule[]): Promise<void> {
  const rules = build(r => r as unknown as Rule);

  const recurse = async (node: Nodes): Promise<void> => {
    if (node.type !== 'root') {
      for (const rule of rules) {
        if (rule.if(node)) await rule.update(node);
      }
    }
    if ('children' in node) {
      for (const child of node.children) {
        await recurse(child);
      }
    }
  };

  await recurse(root);
}
