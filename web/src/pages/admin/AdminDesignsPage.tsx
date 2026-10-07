import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { adminApi } from "../../api/admin";
import type { AdminDesignItem } from "../../api/admin";
import { designsApi } from "../../api/designs";
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

export function AdminDesignsPage() {
  const [filter, setFilter] = useState("all");
  const [page, setPage] = useState(1);
  const [data, setData] = useState<ListResult<AdminDesignItem> | null>(null);
  const [error, setError] = useState("");
  const [acting, setActing] = useState("");

  const load = useCallback(async () => {
    setError("");
    try {
      const q = FILTERS.find((f) => f.key === filter)?.query ?? {};
      setData(await adminApi.designs({ ...q, page }));
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "加载失败,请稍后重试");
    }
  }, [filter, page]);

  useEffect(() => {
    void load();
  }, [load]);

  async function act(item: AdminDesignItem, fn: () => Promise<unknown>) {
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

  const removeItem = (item: AdminDesignItem) => {
    if (!window.confirm(`确定删除 ${item.namespace}/${item.name} 吗?将删除所有版本与文件,不可恢复。`))
      return;
    return act(item, () => designsApi.remove(item.id));
  };

  return (
    <>
      <h1>DESIGN.md</h1>
      <p className="sub">管理全站 DESIGN.md 制品,可标记官方或下架</p>

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
              data.items.map((d) => {
                const st = statusView(d.status);
                const busy = acting === d.id;
                return (
                  <tr key={d.id}>
                    <td>
                      <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
                        <span className="thumb-xs">
                          {d.preview_thumb_url ? (
                            <img src={d.preview_thumb_url} alt="" />
                          ) : (
                            <span
                              style={{
                                width: "100%",
                                height: "100%",
                                display: "flex",
                                alignItems: "center",
                                justifyContent: "center",
                                fontFamily: "var(--serif)",
                                color: "var(--faint)",
                              }}
                            >
                              {d.name.charAt(0).toUpperCase()}
                            </span>
                          )}
                        </span>
                        <span className="mono">
                          <Link to={`/d/${d.namespace}/${d.name}`}>
                            <span className="ns">{d.namespace}/</span>
                            {d.name}
                          </Link>{" "}
                          {d.official && <OfficialBadge />}
                        </span>
                      </div>
                    </td>
                    <td className="muted">
                      {d.owner.nickname || d.owner.username} (@{d.owner.username})
                    </td>
                    <td className="mono">v{d.latest_version}</td>
                    <td className="muted">{formatCount(d.download_count)}</td>
                    <td style={{ whiteSpace: "nowrap" }}>
                      <span className={`status-dot ${st.cls}`}>{st.label}</span>
                    </td>
                    <td style={{ whiteSpace: "nowrap" }}>
                      {d.status === "pending" && (
                        <>
                          <button
                            className="btn btn-outline btn-sm"
                            disabled={busy}
                            onClick={() => act(d, () => adminApi.approveDesign(d.id))}
                          >
                            审核通过
                          </button>{" "}
                        </>
                      )}
                      <button
                        className="btn btn-outline btn-sm"
                        disabled={busy}
                        onClick={() => act(d, () => adminApi.setDesignOfficial(d.id, !d.official))}
                      >
                        {d.official ? "取消官方" : "标记官方"}
                      </button>{" "}
                      <button
                        className="btn btn-outline btn-sm"
                        disabled={busy}
                        onClick={() =>
                          act(d, () =>
                            d.status === "taken_down"
                              ? designsApi.restore(d.id)
                              : designsApi.takedown(d.id),
                          )
                        }
                      >
                        {d.status === "taken_down" ? "恢复" : "下架"}
                      </button>{" "}
                      <button className="btn btn-danger btn-sm" disabled={busy} onClick={() => removeItem(d)}>
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
