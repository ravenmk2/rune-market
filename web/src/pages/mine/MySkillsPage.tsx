import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { skillsApi } from "../../api/skills";
import type { MySkillItem } from "../../api/skills";
import { ApiError } from "../../api/client";
import { useAuth } from "../../context/AuthContext";
import { OfficialBadge } from "../../components/Badge";
import { formatCount, formatDate } from "../../utils/format";

function statusView(status: string): { cls: string; label: string } {
  if (status === "published") return { cls: "active", label: "已发布" };
  if (status === "pending") return { cls: "pending", label: "审核中" };
  return { cls: "disabled", label: "已下架" };
}

export function MySkillsPage() {
  const { user, loading: authLoading } = useAuth();
  const [items, setItems] = useState<MySkillItem[] | null>(null);
  const [error, setError] = useState("");
  const [acting, setActing] = useState("");

  const load = useCallback(async () => {
    setError("");
    try {
      const r = await skillsApi.mine();
      setItems(r.items);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "加载失败,请稍后重试");
    }
  }, []);

  useEffect(() => {
    if (user) void load();
  }, [user, load]);

  async function toggleStatus(item: MySkillItem) {
    setActing(item.id);
    setError("");
    try {
      if (item.status === "taken_down") await skillsApi.restore(item.id);
      else await skillsApi.takedown(item.id);
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

  const pending = items?.filter((s) => s.status === "pending") ?? [];

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
        <Link className="btn btn-primary" to="/publish">
          发布
        </Link>
      </section>

      <nav className="tabs">
        <a className="active">
          Skills{items && <span className="count">{items.length}</span>}
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
            你还没有发布制品,点击右上角「发布」上传第一个 Skill。
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
              {items.map((s) => {
                const st = statusView(s.status);
                return (
                  <tr key={s.id}>
                    <td className="mono">
                      <Link to={`/s/${s.namespace}/${s.name}`}>{s.name}</Link>{" "}
                      {s.official && <OfficialBadge />}
                    </td>
                    <td className="mono">v{s.latest_version}</td>
                    <td>
                      <span className={`status-dot ${st.cls}`}>{st.label}</span>
                    </td>
                    <td>{formatCount(s.download_count)}</td>
                    <td className="muted">{formatDate(s.updated_at)}</td>
                    <td style={{ whiteSpace: "nowrap" }}>
                      <Link className="btn btn-outline btn-sm" to="/publish">
                        新版
                      </Link>{" "}
                      <Link className="btn btn-outline btn-sm" to={`/mine/skills/${s.id}/edit`}>
                        编辑
                      </Link>{" "}
                      <button
                        className="btn btn-outline btn-sm"
                        disabled={acting === s.id}
                        onClick={() => toggleStatus(s)}
                      >
                        {s.status === "taken_down" ? "恢复" : "下架"}
                      </button>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {pending.map((s) => (
        <div className="alert warn" style={{ marginTop: 18 }} key={s.id}>
          <span>!</span>
          <span>
            <b>{s.name}</b> 正在等待管理员审核,审核通过后将公开展示。
          </span>
        </div>
      ))}
    </main>
  );
}
