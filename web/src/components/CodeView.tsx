import { useMemo } from "react";
import { escapeHtml, highlightCode, langForPath } from "./highlight";

const FRONTMATTER_RE = /^---\r?\n[\s\S]*?\r?\n---/;

/** 文本预览:Prism 按扩展名高亮,SKILL.md frontmatter 保留底色(输出均已转义,§13) */
export function CodeView({ content, path }: { content: string; path?: string }) {
  const html = useMemo(() => {
    const lang = path ? langForPath(path) : null;
    const m = content.match(FRONTMATTER_RE);
    const fm = m ? m[0] : "";
    const body = fm ? content.slice(fm.length) : content;
    const head = fm ? `<span class="frontmatter">${escapeHtml(fm)}</span>` : "";
    return head + (lang ? highlightCode(body, lang) : escapeHtml(body));
  }, [content, path]);
  return <div className="codebox" dangerouslySetInnerHTML={{ __html: html }} />;
}
