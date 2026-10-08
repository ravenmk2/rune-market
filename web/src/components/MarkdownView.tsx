import { useEffect, useMemo, useRef } from "react";
import { marked } from "marked";
import type { Tokens } from "marked";
import DOMPurify from "dompurify";
import { escapeHtml, highlightCode, resolveLang } from "./highlight";

const HEX_RE = /#(?:[0-9a-fA-F]{6}|[0-9a-fA-F]{3})\b/;

// 代码块走 Prism 高亮(输出已转义,之后统一经 DOMPurify)
marked.use({
  renderer: {
    code({ text, lang }: Tokens.Code) {
      const l = resolveLang(lang ?? "");
      const cls = l ? ` class="language-${l}"` : "";
      const inner = l ? highlightCode(text, l) : escapeHtml(text);
      return `<pre><code${cls}>${inner}</code></pre>`;
    },
  },
});

/**
 * DESIGN.md 渲染:marked → DOMPurify sanitize(§13 强制)。
 * 渲染后对文本中的 hex 色值加行内色块(§10 可选增强;只操作文本节点,不注入 HTML)。
 */
export function MarkdownView({ markdown }: { markdown: string }) {
  const html = useMemo(
    () => DOMPurify.sanitize(marked.parse(markdown, { async: false })),
    [markdown],
  );
  const ref = useRef<HTMLElement>(null);

  useEffect(() => {
    const root = ref.current;
    if (!root) return;
    const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT, {
      acceptNode: (node) =>
        HEX_RE.test(node.nodeValue ?? "") && !node.parentElement?.closest("pre,code")
          ? NodeFilter.FILTER_ACCEPT
          : NodeFilter.FILTER_REJECT,
    });
    const targets: Text[] = [];
    while (walker.nextNode()) targets.push(walker.currentNode as Text);

    for (const node of targets) {
      const text = node.nodeValue ?? "";
      const re = new RegExp(HEX_RE, "g");
      const frag = document.createDocumentFragment();
      let last = 0;
      let m: RegExpExecArray | null;
      while ((m = re.exec(text))) {
        frag.append(text.slice(last, m.index));
        const chip = document.createElement("span");
        chip.className = "hex-chip";
        const box = document.createElement("span");
        box.className = "box";
        box.style.background = m[0];
        chip.append(box, document.createTextNode(m[0]));
        frag.append(chip);
        last = m.index + m[0].length;
      }
      frag.append(text.slice(last));
      node.replaceWith(frag);
    }
  }, [html]);

  return <article ref={ref} className="md" dangerouslySetInnerHTML={{ __html: html }} />;
}
