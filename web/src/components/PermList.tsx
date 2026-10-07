import type { Permission } from "../api/skills";

/** allowed-tools 权限公示(§9:警示风险等级) */
export function PermList({ permissions }: { permissions: Permission[] }) {
  if (!permissions.length) return <p className="muted" style={{ fontSize: 13.5 }}>无权限要求</p>;
  return (
    <div className="perm-list">
      {permissions.map((p, i) => (
        <div className="perm" key={`${p.tool}-${i}`}>
          <span className={`dot ${p.risk === "warn" ? "warn" : "ok"}`}></span>
          {p.tool}
          {p.note && <span className="note">{p.note}</span>}
        </div>
      ))}
    </div>
  );
}
