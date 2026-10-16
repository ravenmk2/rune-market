import type { ValidateCheck } from "../api/skills";

export function ValidationSummary({ checks, busy, error, blocked, detailsId }: {
  checks?: ValidateCheck[];
  busy: boolean;
  error?: string;
  blocked?: boolean;
  detailsId: string;
}) {
  const errors = checks?.filter((c) => c.level === "error").length ?? 0;
  const warnings = checks?.filter((c) => c.level === "warn").length ?? 0;
  const message = busy ? "正在上传并校验文件…" : error ? error : !checks ? "选择文件后自动校验。" :
    errors ? `发现 ${errors} 项错误，请修正后重新上传。` : blocked ? "校验已完成，请查看提示后继续。" :
    warnings ? `校验通过，有 ${warnings} 项建议，不影响发布。` : "校验通过，可以继续填写并发布。";
  return <div className={`validation-summary${error || errors ? " has-error" : ""}`} role="status" aria-live="polite">
    <span>{message}</span>
    {(checks || error) && !busy && <button type="button" className="text-button" onClick={() => {
      const target = document.getElementById(detailsId);
      target?.scrollIntoView({ behavior: "smooth", block: "start" });
      target?.focus({ preventScroll: true });
    }}>查看检查项</button>}
  </div>;
}
