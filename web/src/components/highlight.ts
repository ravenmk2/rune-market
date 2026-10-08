import Prism from "prismjs";
import "prismjs/components/prism-markup";
import "prismjs/components/prism-css";
import "prismjs/components/prism-clike";
import "prismjs/components/prism-javascript";
import "prismjs/components/prism-typescript";
import "prismjs/components/prism-jsx";
import "prismjs/components/prism-tsx";
import "prismjs/components/prism-json";
import "prismjs/components/prism-yaml";
import "prismjs/components/prism-bash";
import "prismjs/components/prism-go";
import "prismjs/components/prism-python";
import "prismjs/components/prism-sql";
import "prismjs/components/prism-docker";
import "prismjs/components/prism-toml";
import "prismjs/components/prism-markdown";
import "prismjs/components/prism-ini";
import "prismjs/components/prism-diff";

/** 扩展名 → Prism 语言(小写、不含点) */
const EXT_LANG: Record<string, string> = {
  html: "markup",
  xml: "markup",
  svg: "markup",
  css: "css",
  js: "javascript",
  mjs: "javascript",
  cjs: "javascript",
  ts: "typescript",
  mts: "typescript",
  cts: "typescript",
  jsx: "jsx",
  tsx: "tsx",
  json: "json",
  yaml: "yaml",
  yml: "yaml",
  sh: "bash",
  bash: "bash",
  zsh: "bash",
  go: "go",
  py: "python",
  sql: "sql",
  dockerfile: "docker",
  toml: "toml",
  md: "markdown",
  markdown: "markdown",
  ini: "ini",
  cfg: "ini",
  conf: "ini",
  properties: "ini",
  diff: "diff",
  patch: "diff",
};

/** markdown 代码块语言标识别名 → Prism 语言 */
const LANG_ALIAS: Record<string, string> = {
  js: "javascript",
  mjs: "javascript",
  cjs: "javascript",
  ts: "typescript",
  mts: "typescript",
  cts: "typescript",
  py: "python",
  sh: "bash",
  shell: "bash",
  zsh: "bash",
  console: "bash",
  yml: "yaml",
  html: "markup",
  xml: "markup",
  svg: "markup",
  vue: "markup",
  dockerfile: "docker",
  patch: "diff",
};

/** 由文件路径推断 Prism 语言,未知返回 null */
export function langForPath(path: string): string | null {
  const base = path.split("/").pop()?.toLowerCase() ?? "";
  if (base === "dockerfile") return "docker";
  const dot = base.lastIndexOf(".");
  if (dot <= 0) return null;
  const lang = EXT_LANG[base.slice(dot + 1)];
  return lang && Prism.languages[lang] ? lang : null;
}

/** 由 markdown 代码块语言标识推断 Prism 语言,未知返回 null */
export function resolveLang(lang: string): string | null {
  const id = lang.trim().split(/\s+/)[0].toLowerCase();
  if (!id) return null;
  const mapped = LANG_ALIAS[id] ?? id;
  return Prism.languages[mapped] ? mapped : null;
}

export function escapeHtml(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

/** Prism 高亮,返回安全 HTML(无对应语言时纯转义) */
export function highlightCode(code: string, lang: string): string {
  const grammar = Prism.languages[lang];
  if (!grammar) return escapeHtml(code);
  return Prism.highlight(code, grammar, lang);
}
