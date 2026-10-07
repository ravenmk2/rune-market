import type { ValidateCheck } from "../api/skills";

const MARKS: Record<ValidateCheck["level"], { cls: string; mark: string }> = {
  ok: { cls: "ok", mark: "✓" },
  warn: { cls: "warn", mark: "!" },
  error: { cls: "err", mark: "✗" },
};

/** 校验报告列表(§9:ok/warn/error 三级) */
export function CheckList({ checks }: { checks: ValidateCheck[] }) {
  return (
    <div className="check-list">
      {checks.map((c, i) => {
        const m = MARKS[c.level] ?? MARKS.ok;
        return (
          <div className="check" key={i}>
            <span className={`mark ${m.cls}`}>{m.mark}</span>
            <div>
              <div className="c-title">{c.title}</div>
              {c.detail && <div className="c-detail">{c.detail}</div>}
            </div>
          </div>
        );
      })}
    </div>
  );
}
