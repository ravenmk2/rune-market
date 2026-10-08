/** 格式化工具 */

export function formatBytes(n: number): string {
  if (!Number.isFinite(n) || n < 0) return "-";
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(n < 1024 * 100 ? 1 : 0)} KB`;
  return `${(n / 1024 / 1024).toFixed(1)} MB`;
}

/** ISO 时间 → YYYY-MM-DD */
export function formatDate(iso?: string): string {
  if (!iso) return "-";
  return iso.slice(0, 10);
}

/** 下载量缩写:12437 → 12.4k */
export function formatCount(n: number): string {
  if (n >= 1000) return `${(n / 1000).toFixed(1)}k`;
  return String(n);
}

/** sha256 截断:a3f9…c21e */
export function shortSha(sha?: string): string {
  if (!sha) return "-";
  if (sha.length <= 18) return sha;
  return `${sha.slice(0, 7)}…${sha.slice(-7)}`;
}

const HARNESS_LABELS: Record<string, string> = {
  "claude-code": "Claude Code",
  codex: "Codex",
  cursor: "Cursor",
};

export function harnessLabel(id: string): string {
  return HARNESS_LABELS[id] ?? id;
}
