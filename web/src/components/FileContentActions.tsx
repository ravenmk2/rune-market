import { useEffect, useRef, useState } from "react";
import { MarkdownModeSwitch } from "./MarkdownModeSwitch";

async function copyText(text: string) {
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text);
      return;
    }
  } catch { /* Fall back for browsers without clipboard permission. */ }
  const previous = document.activeElement;
  const selection = window.getSelection();
  const ranges = selection ? Array.from({ length: selection.rangeCount }, (_, i) => selection.getRangeAt(i).cloneRange()) : [];
  const input = document.createElement("textarea");
  input.value = text;
  input.readOnly = true;
  input.style.cssText = "position:fixed;left:-9999px;top:0;opacity:0";
  document.body.appendChild(input);
  try {
    input.select();
    if (!document.execCommand("copy")) throw new Error("Copy failed");
  } finally {
    input.remove();
    if (previous instanceof HTMLElement) previous.focus({ preventScroll: true });
    if (selection) {
      selection.removeAllRanges();
      ranges.forEach((range) => selection.addRange(range));
    }
  }
}

export function FileContentActions({ content, markdown, mode, onModeChange, wrap, onWrapChange, binary = false }: {
  content?: string;
  markdown: boolean;
  mode: "preview" | "raw";
  onModeChange: (mode: "preview" | "raw") => void;
  wrap: boolean;
  onWrapChange: (value: boolean) => void;
  binary?: boolean;
}) {
  const [status, setStatus] = useState<"idle" | "copying" | "copied" | "failed">("idle");
  const generation = useRef(0);
  useEffect(() => () => { generation.current += 1; }, []);
  useEffect(() => {
    if (status !== "copied") return;
    const timer = window.setTimeout(() => setStatus("idle"), 2000);
    return () => window.clearTimeout(timer);
  }, [status]);

  async function copy() {
    if (content === undefined) return;
    const current = generation.current;
    setStatus("copying");
    try {
      await copyText(content);
      if (generation.current === current) setStatus("copied");
    } catch {
      if (generation.current === current) setStatus("failed");
    }
  }

  const wrapDisabled = markdown && mode === "preview";
  const wrapLabel = wrapDisabled ? "自动换行：仅源码模式可用" : `自动换行：${wrap ? "开启" : "关闭"}`;
  const copyLabel = status === "copied" ? "已复制" : status === "copying" ? "复制中…" : "复制源码";

  return <div className="file-content-actions">
    {markdown && <MarkdownModeSwitch mode={mode} onChange={onModeChange} />}
    {!binary && <>
      {markdown && <span className="file-actions-divider" aria-hidden="true" />}
      <button type="button" className="btn btn-outline file-action-icon"
        title={wrapLabel} aria-label={wrapLabel} aria-disabled={wrapDisabled} aria-pressed={!wrapDisabled && wrap}
        onClick={() => { if (!wrapDisabled) onWrapChange(!wrap); }}>
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"
          strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" focusable="false">
          <path d="M3 6h18M3 11h14a4 4 0 0 1 0 8h-5m3-3-3 3 3 3M3 16h4" />
        </svg>
      </button>
      <button type="button" className="btn btn-outline file-action-icon" title={copyLabel} aria-label={copyLabel}
        disabled={content === undefined || status === "copying"} onClick={copy}>
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"
          strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" focusable="false">
          {status === "copied" ? <path d="m5 12 4 4L19 6" /> : <>
            <rect x="8" y="8" width="12" height="12" rx="2" />
            <path d="M16 8V4a2 2 0 0 0-2-2H4a2 2 0 0 0-2 2v10a2 2 0 0 0 2 2h4" />
          </>}
        </svg>
      </button>
      <span role="status" className={status === "failed" ? "copy-error" : "sr-only"}>
        {status === "failed" ? "复制失败，请手动选择内容复制" : status === "copied" ? "已复制" : ""}
      </span>
    </>}
  </div>;
}
