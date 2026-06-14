import { components } from "@/components/docs/markdown/components";
import type { Section } from "@/lib/toc";
import { cn } from "@/lib/utils";
import { allPosts } from "content-collections";
import Markdown from "react-markdown";
import { Link, useLoaderData } from "react-router";
import rehypeUnwrapImages from "rehype-unwrap-images";
import type { Route } from "./+types/Main";

export function clientLoader({ params }: Route.ClientLoaderArgs) {
  console.log(allPosts);
  const post = allPosts.find(
    (post) =>
      post._meta.directory === params.collection &&
      post._meta.fileName.replace(".md", "") === params.slug,
  );
  if (!post) throw new Error("Post not found");
  return { post };
}

export default function Home() {
  const { post } = useLoaderData<typeof clientLoader>();

  return (
    <div className="w-full flex justify-center items-start gap-4 px-5 py-8">
      <TableOfContents toc={post.toc} />
      <div className="max-w-3xl w-full px-5">
        <Markdown rehypePlugins={[rehypeUnwrapImages]} components={components}>
          {post.content}
        </Markdown>
      </div>
    </div>
  );
}

const SectionItem = ({
  section,
  depth = 0,
}: {
  section: Section;
  depth?: number;
}) => {
  return (
    <li className={cn(depth > 0 && "pl-4")}>
      <Link
        to={{ hash: section.url }}
        className="block text-sm leading-snug text-[#43251a] hover:opacity-80"
      >
        {section.title}
      </Link>
      {section.sections?.length ? (
        <ul className="mt-2 space-y-2 list-none">
          {section.sections.map((child) => (
            <SectionItem key={child.url} section={child} depth={depth + 1} />
          ))}
        </ul>
      ) : null}
    </li>
  );
};

const TableOfContents = ({ toc }: { toc: Section[] }) => {
  if (!toc.length) return null;

  return (
    <aside className="w-fit mt-8 max-w-xs sticky top-6">
      <p className="mb-2 text-xs uppercase tracking-wide font-semibold text-[#8b7b74]">
        Contents
      </p>
      <ul className="space-y-4 list-none">
        {toc.map((section) => (
          <SectionItem key={section.url} section={section} />
        ))}
      </ul>
    </aside>
  );
};
