const FRONTMATTER_RE = /^---\r?\n[\s\S]*?\r?\n---/;

/** 文本预览:等宽直出,SKILL.md frontmatter 高亮(不引入高亮库,§13 按纯文本渲染防 XSS) */
export function CodeView({ content }: { content: string }) {
  const m = content.match(FRONTMATTER_RE);
  if (!m) return <div className="codebox">{content}</div>;
  return (
    <div className="codebox">
      <span className="frontmatter">{m[0]}</span>
      {content.slice(m[0].length)}
    </div>
  );
}
