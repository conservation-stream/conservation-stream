import { cloneElement, type JSX } from 'react';

import { cn } from '@/lib/utils';
import { Video } from '../video/Video';

type ElementProps<TTag extends keyof JSX.IntrinsicElements> = JSX.IntrinsicElements[TTag];

const videoExtensions = new Set(['.mp4', '.webm', '.ogg', '.mov', '.m4v', '.m3u8']);

const getExtension = (src?: string) => {
  if (!src) return '';
  const normalized = src.split('?')[0]?.split('#')[0] ?? '';
  const lastDotIndex = normalized.lastIndexOf('.');
  if (lastDotIndex === -1) return '';
  return normalized.slice(lastDotIndex).toLowerCase();
};

const TextLikeClassName = 'w-full max-w-2xl mx-auto';

const MarkdownImage = ({ src, alt, className, ...props }: ElementProps<'img'>) => {
  if (!src) return;
  const extension = getExtension(src);
  const url = new URL(src, window.location.origin);
  const aspectRatio = url.searchParams.get('aspect-ratio');

  if (extension && videoExtensions.has(extension)) {
    return (
      <div className="my-3 w-full">
        <Video src={src} />
        <p className="text-sm text-gray-500 text-center mt-2">{alt}</p>
      </div>
    );
  }
  return (
    <img
      src={src}
      alt={alt}
      className={cn('w-full h-auto rounded-lg my-3', className)}
      style={{
        aspectRatio: aspectRatio ? `1/${aspectRatio}` : undefined
      }}
      {...props}
    />
  );
};

const MarkdownOrderedList = ({ children, className, ...props }: ElementProps<'ol'>) => (
  <ol className={cn(TextLikeClassName, 'list-inside list-decimal whitespace-normal text-[15px] leading-relaxed in-[li]:pl-6', className)} data-streamdown="ordered-list" {...props}>
    {children}
  </ol>
);

const MarkdownListItem = ({ children, className, ...props }: ElementProps<'li'>) => (
  <li className={cn('py-1 [&>p]:inline', className)} data-streamdown="list-item" {...props}>
    {children}
  </li>
);

const MarkdownUnorderedList = ({ children, className, ...props }: ElementProps<'ul'>) => (
  <ul className={cn(TextLikeClassName, 'list-inside list-disc whitespace-normal text-[15px] leading-relaxed in-[li]:pl-6', className)} data-streamdown="unordered-list" {...props}>
    {children}
  </ul>
);

const MarkdownHorizontalRule = ({ className, ...props }: ElementProps<'hr'>) => <hr className={cn('my-6 border-border', className)} data-streamdown="horizontal-rule" {...props} />;

const MarkdownStrong = ({ children, className, ...props }: ElementProps<'span'>) => (
  <span className={cn('font-semibold', className)} data-streamdown="strong" {...props}>
    {children}
  </span>
);

const MarkdownLink = ({ children, className, ...props }: ElementProps<'a'>) => (
  <a className={cn('wrap-anywhere font-medium text-primary underline', className)} data-streamdown="link" rel="noreferrer" target="_blank" {...props}>
    {children}
  </a>
);

const MarkdownH1 = ({ children, className, ...props }: ElementProps<'h1'>) => (
  <h1 className={cn(TextLikeClassName, 'mt-6 mb-1 font-semibold text-3xl', className)} data-streamdown="heading-1" {...props}>
    {children}
  </h1>
);

const MarkdownH2 = ({ children, className, ...props }: ElementProps<'h2'>) => (
  <h2 className={cn(TextLikeClassName, 'mt-6 mb-1 font-semibold text-2xl', className)} data-streamdown="heading-2" {...props}>
    {children}
  </h2>
);

const MarkdownH3 = ({ children, className, ...props }: ElementProps<'h3'>) => (
  <h3 className={cn(TextLikeClassName, 'mt-6 mb-1 font-semibold text-xl', className)} data-streamdown="heading-3" {...props}>
    {children}
  </h3>
);

const MarkdownH4 = ({ children, className, ...props }: ElementProps<'h4'>) => (
  <h4 className={cn(TextLikeClassName, 'mt-6 mb-1 font-semibold text-lg', className)} data-streamdown="heading-4" {...props}>
    {children}
  </h4>
);

const MarkdownH5 = ({ children, className, ...props }: ElementProps<'h5'>) => (
  <h5 className={cn(TextLikeClassName, 'mt-6 mb-1 font-semibold text-base', className)} data-streamdown="heading-5" {...props}>
    {children}
  </h5>
);

const MarkdownH6 = ({ children, className, ...props }: ElementProps<'h6'>) => (
  <h6 className={cn(TextLikeClassName, 'mt-6 mb-2 font-semibold text-sm', className)} data-streamdown="heading-6" {...props}>
    {children}
  </h6>
);

const MarkdownTable = ({ children, className, ...props }: ElementProps<'table'>) => (
  <table className={className} {...props}>
    {children}
  </table>
);

const MarkdownTableHead = ({ children, className, ...props }: ElementProps<'thead'>) => (
  <thead className={cn('bg-muted/80', className)} {...props}>
    {children}
  </thead>
);

const MarkdownTableBody = ({ children, className, ...props }: ElementProps<'tbody'>) => (
  <tbody className={cn('divide-y divide-border bg-muted/40', className)} {...props}>
    {children}
  </tbody>
);

const MarkdownTableRow = ({ children, className, ...props }: ElementProps<'tr'>) => (
  <tr className={cn('border-border border-b', className)} {...props}>
    {children}
  </tr>
);

const MarkdownTableHeaderCell = ({ children, className, ...props }: ElementProps<'th'>) => (
  <th className={cn('whitespace-nowrap px-4 py-2 text-left font-semibold text-sm', className)} {...props}>
    {children}
  </th>
);

const MarkdownTableCell = ({ children, className, ...props }: ElementProps<'td'>) => (
  <td className={cn('px-4 py-2 text-sm', className)} {...props}>
    {children}
  </td>
);

const MarkdownBlockquote = ({ children, className, ...props }: ElementProps<'blockquote'>) => (
  <blockquote className={cn('my-4 border-muted-foreground/30 border-l-4 pl-4 text-muted-foreground italic', className)} data-streamdown="blockquote" {...props}>
    {children}
  </blockquote>
);

const MarkdownInlineCode = ({ children, className, ...props }: ElementProps<'code'>) => (
  <code className={cn('rounded bg-muted px-1.5 py-0.5 font-mono text-sm', className)} data-streamdown="inline-code" {...props}>
    {children}
  </code>
);

const MarkdownPre = ({ children }: ElementProps<'pre'>) => {
  if (children && typeof children === 'object') {
    return cloneElement(children as JSX.Element, { 'data-block': 'true' });
  }
  return children;
};

const MarkdownSuperscript = ({ children, className, ...props }: ElementProps<'sup'>) => (
  <sup className={cn('text-sm', className)} {...props}>
    {children}
  </sup>
);

const MarkdownSubscript = ({ children, className, ...props }: ElementProps<'sub'>) => (
  <sub className={cn('text-sm', className)} {...props}>
    {children}
  </sub>
);

const MarkdownParagraph = ({ children, className, ...props }: ElementProps<'p'>) => (
  <p className={cn(TextLikeClassName, 'pb-4 pt-1 sm:text-[15px] leading-relaxed', className)} {...props}>
    {children}
  </p>
);

const MarkdownSection = ({ children, className, ...props }: ElementProps<'section'>) => (
  <section className={className} {...props}>
    {children}
  </section>
);

export const components = {
  img: MarkdownImage,
  ol: MarkdownOrderedList,
  li: MarkdownListItem,
  ul: MarkdownUnorderedList,
  hr: MarkdownHorizontalRule,
  strong: MarkdownStrong,
  a: MarkdownLink,
  h1: MarkdownH1,
  h2: MarkdownH2,
  h3: MarkdownH3,
  h4: MarkdownH4,
  h5: MarkdownH5,
  h6: MarkdownH6,
  table: MarkdownTable,
  thead: MarkdownTableHead,
  tbody: MarkdownTableBody,
  tr: MarkdownTableRow,
  th: MarkdownTableHeaderCell,
  td: MarkdownTableCell,
  blockquote: MarkdownBlockquote,
  code: MarkdownInlineCode,
  pre: MarkdownPre,
  sup: MarkdownSuperscript,
  sub: MarkdownSubscript,
  p: MarkdownParagraph,
  section: MarkdownSection
};
