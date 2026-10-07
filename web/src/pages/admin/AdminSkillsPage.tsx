import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { adminApi } from "../../api/admin";
import type { AdminSkillItem } from "../../api/admin";
import { skillsApi } from "../../api/skills";
import type { ListResult, Query } from "../../api/client";
import { ApiError } from "../../api/client";
import { OfficialBadge } from "../../components/Badge";
import { Pagination } from "../../components/Pagination";
import { formatCount } from "../../utils/format";

const FILTERS: { key: string; label: string; query: Query }[] = [
  { key: "all", label: "全部", query: {} },
  { key: "official", label: "✦ 仅官方", query: { official: true } },
  { key: "pending", label: "审核中", query: { status: "pending" } },
  { key: "taken_down", label: "已下架", query: { status: "taken_down" } },
];

function statusView(status: string): { cls: string; label: string } {
  if (status === "pending") return { cls: "pending", label: "审核中" };
  if (status === "taken_down") return { cls: "disabled", label: "已下架" };
  return { cls: "active", label: "已发布" };
}

export function AdminSkillsPage() {
  const [filter, setFilter] = useState("all");
  const [page, setPage] = useState(1);
  const [data, setData] = useState<ListResult<AdminSkillItem> | null>(null);
  const [error, setError] = useState("");
  const [acting, setActing] = useState("");

  const load = useCallback(async () => {
    setError("");
    try {
      const q = FILTERS.find((f) => f.key === filter)?.query ?? {};
      setData(await adminApi.skills({ ...q, page }));
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "加载失败,请稍后重试");
    }
  }, [filter, page]);

  useEffect(() => {
    void load();
  }, [load]);

  async function act(item: AdminSkillItem, fn: () => Promise<unknown>) {
    setActing(item.id);
    setError("");
    try {
      await fn();
      await load();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "操作失败,请稍后重试");
    } finally {
      setActing("");
    }
  }

  const removeItem = (item: AdminSkillItem) => {
    if (!window.confirm(`确定删除 ${item.namespace}/${item.name} 吗?将删除所有版本与文件,不可恢复。`))
      return;
    return act(item, () => skillsApi.remove(item.id));
  };

  return (
    <>
      <h1>Skills</h1>
      <p className="sub">管理全站 Skill 制品,可标记官方或下架</p>

      <div className="alert ok" style={{ marginBottom: 18 }}>
        官方制品会在列表页置顶并带 Official 徽标,命名空间归属 RuneMarket。
      </div>

      <div className="filter-row" style={{ margin: "0 0 18px" }}>
        {FILTERS.map((f) => (
          <button
            key={f.key}
            className={`chip${filter === f.key ? " active" : ""}`}
            onClick={() => {
              setPage(1);
              setFilter(f.key);
            }}
          >
            {f.label}
          </button>
        ))}
      </div>

      {error && (
        <div className="alert err" style={{ marginBottom: 18 }}>
          <span>!</span>
          <span>{error}</span>
        </div>
      )}

      <section className="panel">
        <table className="table">
          <thead>
            <tr>
              <th>名称</th>
              <th>发布者</th>
              <th style={{ whiteSpace: "nowrap" }}>最新版本</th>
              <th style={{ whiteSpace: "nowrap" }}>下载量</th>
              <th style={{ whiteSpace: "nowrap" }}>状态</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {!data ? (
              <tr>
                <td colSpan={6} className="muted" style={{ textAlign: "center" }}>
                  加载中…
                </td>
              </tr>
            ) : data.items.length === 0 ? (
              <tr>
                <td colSpan={6} className="muted" style={{ textAlign: "center" }}>
                  没有匹配的制品。
                </td>
              </tr>
            ) : (
              data.items.map((s) => {
                const st = statusView(s.status);
                const busy = acting === s.id;
                return (
                  <tr key={s.id}>
                    <td className="mono">
                      <Link to={`/s/${s.namespace}/${s.name}`}>
                        <span className="ns">{s.namespace}/</span>
                        {s.name}
                      </Link>{" "}
                      {s.official && <OfficialBadge />}
                    </td>
                    <td className="muted">
                      {s.owner.nickname || s.owner.username} (@{s.owner.username})
                    </td>
                    <td className="mono">v{s.latest_version}</td>
                    <td className="muted">{formatCount(s.download_count)}</td>
                    <td style={{ whiteSpace: "nowrap" }}>
                      <span className={`status-dot ${st.cls}`}>{st.label}</span>
                    </td>
                    <td style={{ whiteSpace: "nowrap" }}>
                      {s.status === "pending" && (
                        <>
                          <button
                            className="btn btn-outline btn-sm"
                            disabled={busy}
                            onClick={() => act(s, () => adminApi.approveSkill(s.id))}
                          >
                            审核通过
                          </button>{" "}
                        </>
                      )}
                      <button
                        className="btn btn-outline btn-sm"
                        disabled={busy}
                        onClick={() => act(s, () => adminApi.setSkillOfficial(s.id, !s.official))}
                      >
                        {s.official ? "取消官方" : "标记官方"}
                      </button>{" "}
                      <button
                        className="btn btn-outline btn-sm"
                        disabled={busy}
                        onClick={() =>
                          act(s, () =>
                            s.status === "taken_down"
                              ? skillsApi.restore(s.id)
                              : skillsApi.takedown(s.id),
                          )
                        }
                      >
                        {s.status === "taken_down" ? "恢复" : "下架"}
                      </button>{" "}
                      <button className="btn btn-danger btn-sm" disabled={busy} onClick={() => removeItem(s)}>
                        删除
                      </button>
                    </td>
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
      </section>

      {data && (
        <Pagination page={data.page} pageSize={data.page_size} total={data.total} onChange={setPage} />
      )}
    </>
  );
}
