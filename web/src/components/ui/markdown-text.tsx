"use client";

import type { ComponentPropsWithoutRef, ReactNode } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

export function MarkdownText({
  content,
  className = "",
}: {
  content: string;
  className?: string;
}) {
  type AnchorProps = ComponentPropsWithoutRef<"a">;
  type BlockProps = { children?: ReactNode };
  type CodeProps = { children?: ReactNode };

  return (
    <div className={`min-w-0 ${className}`.trim()}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          p: ({ children }: BlockProps) => <p className="mb-3 last:mb-0">{children}</p>,
          ul: ({ children }: BlockProps) => <ul className="mb-3 list-disc space-y-1 pl-5 last:mb-0">{children}</ul>,
          ol: ({ children }: BlockProps) => <ol className="mb-3 list-decimal space-y-1 pl-5 last:mb-0">{children}</ol>,
          li: ({ children }: BlockProps) => <li>{children}</li>,
          a: ({ href, children }: AnchorProps) => (
            <a
              href={href}
              target="_blank"
              rel="noreferrer"
              className="underline decoration-[var(--color-editorial-line)] underline-offset-2 hover:opacity-80"
            >
              {children}
            </a>
          ),
          code: ({ children }: CodeProps) => (
            <code className="rounded bg-[var(--color-editorial-panel-strong)] px-1.5 py-0.5 text-[0.92em]">{children}</code>
          ),
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
}
