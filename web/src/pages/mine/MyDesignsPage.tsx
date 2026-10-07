import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { designsApi } from "../../api/designs";
import type { DesignListItem } from "../../api/designs";
import { ApiError } from "../../api/client";
import { useAuth } from "../../context/AuthContext";
import { OfficialBadge } from "../../components/Badge";
import { formatCount, formatDate } from "../../utils/format";

function statusView(status: string): { cls: string; label: string } {
  if (status === "published") return { cls: "active", label: "已发布" };
  if (status === "pending") return { cls: "pending", label: "审核中" };
  return { cls: "disabled", label: "已下架" };
}

export function MyDesignsPage() {
  const { user, loading: authLoading } = useAuth();
  const [items, setItems] = useState<DesignListItem[] | null>(null);
  const [error, setError] = useState("");
  const [acting, setActing] = useState("");

  const load = useCallback(async () => {
    setError("");
    try {
      const r = await designsApi.mine();
      setItems(r.items);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "加载失败,请稍后重试");
    }
  }, []);

  useEffect(() => {
    if (user) void load();
  }, [user, load]);

  async function toggleStatus(item: DesignListItem) {
    setActing(item.id);
    setError("");
    try {
      if (item.status === "taken_down") await designsApi.restore(item.id);
      else await designsApi.takedown(item.id);
      await load();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "操作失败,请稍后重试");
    } finally {
      setActing("");
    }
  }

  if (!authLoading && !user) {
    return (
      <main className="container">
        <section className="page-head">
          <h1>我的发布</h1>
          <p className="sub">管理你发布的 Skills 与 DESIGN.md。</p>
        </section>
        <div className="alert warn" style={{ marginTop: 20 }}>
          <span>!</span>
          <span>
            请先<Link to="/login" style={{ fontWeight: 600 }}>登录</Link>后查看你的制品。
          </span>
        </div>
      </main>
    );
  }

  const pending = items?.filter((d) => d.status === "pending") ?? [];

  return (
    <main className="container">
      <section
        className="page-head"
        style={{
          display: "flex",
          alignItems: "flex-end",
          justifyContent: "space-between",
          gap: 16,
          flexWrap: "wrap",
        }}
      >
        <div>
          <h1>我的发布</h1>
          <p className="sub">管理你发布的 Skills 与 DESIGN.md。</p>
        </div>
        <Link className="btn btn-primary" to="/publish/design">
          发布
        </Link>
      </section>

      <nav className="tabs">
        <Link to="/mine/skills">Skills</Link>
        <a className="active">
          DESIGN.md{items && <span className="count">{items.length}</span>}
        </a>
      </nav>

      {error && (
        <div className="alert err" style={{ marginTop: 20 }}>
          <span>!</span>
          <span>{error}</span>
        </div>
      )}

      {!items ? (
        <p className="muted" style={{ padding: "40px 0", textAlign: "center" }}>
          加载中…
        </p>
      ) : items.length === 0 ? (
        <div
          className="panel panel-pad"
          style={{ textAlign: "center", padding: "60px 24px", marginTop: 24 }}
        >
          <p className="muted">
            你还没有发布制品,点击右上角「发布」上传第一个 DESIGN.md。
          </p>
        </div>
      ) : (
        <div className="panel" style={{ marginTop: 24, overflowX: "auto" }}>
          <table className="table">
            <thead>
              <tr>
                <th>名称</th>
                <th>最新版本</th>
                <th>状态</th>
                <th>下载量</th>
                <th>更新时间</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {items.map((d) => {
                const st = statusView(d.status);
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
                          <Link to={`/d/${d.namespace}/${d.name}`}>{d.name}</Link>{" "}
                          {d.official && <OfficialBadge />}
                        </span>
                      </div>
                    </td>
                    <td className="mono">v{d.latest_version}</td>
                    <td>
                      <span className={`status-dot ${st.cls}`}>{st.label}</span>
                    </td>
                    <td>{formatCount(d.download_count)}</td>
                    <td className="muted">{formatDate(d.updated_at)}</td>
                    <td style={{ whiteSpace: "nowrap" }}>
                      <Link className="btn btn-outline btn-sm" to="/publish/design">
                        新版
                      </Link>{" "}
                      <Link className="btn btn-outline btn-sm" to={`/mine/designs/${d.id}/edit`}>
                        编辑
                      </Link>{" "}
                      <button
                        className="btn btn-outline btn-sm"
                        disabled={acting === d.id}
                        onClick={() => toggleStatus(d)}
                      >
                        {d.status === "taken_down" ? "恢复" : "下架"}
                      </button>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {pending.map((d) => (
        <div className="alert warn" style={{ marginTop: 18 }} key={d.id}>
          <span>!</span>
          <span>
            <b>{d.name}</b> 正在等待管理员审核,审核通过后将公开展示。
          </span>
        </div>
      ))}
    </main>
  );
}
