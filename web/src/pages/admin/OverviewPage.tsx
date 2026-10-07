import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { adminApi } from "../../api/admin";
import type { AdminOverview } from "../../api/admin";
import { ApiError } from "../../api/client";
import { formatBytes } from "../../utils/format";

const REG_MODE_LABELS: Record<string, string> = {
  open: "开放注册",
  approval: "审核注册",
  closed: "关闭注册",
};

export function OverviewPage() {
  const [data, setData] = useState<AdminOverview | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    adminApi
      .overview()
      .then((r) => !cancelled && setData(r))
      .catch((e) => !cancelled && setError(e instanceof ApiError ? e.message : "加载失败,请稍后重试"));
    return () => {
      cancelled = true;
    };
  }, []);

  if (error) {
    return (
      <div className="alert err">
        <span>!</span>
        <span>{error}</span>
      </div>
    );
  }
  if (!data) return <p className="muted">加载中…</p>;

  const todos = [
    { num: data.todos.pending_users, lbl: "待审核注册", to: "/admin/users" },
    { num: data.todos.pending_skills, lbl: "待审核 Skills", to: "/admin/skills" },
    { num: data.todos.pending_designs, lbl: "待审核 DESIGN.md", to: "/admin/designs" },
  ];

  return (
    <>
      <h1>概览</h1>
      <p className="sub">系统概览与待办事项</p>

      <div className="stat-grid">
        <div className="stat">
          <div className="num">{data.stats.users.toLocaleString()}</div>
          <div className="lbl">用户</div>
        </div>
        <div className="stat">
          <div className="num">{data.stats.skills.toLocaleString()}</div>
          <div className="lbl">Skills</div>
        </div>
        <div className="stat">
          <div className="num">{data.stats.designs.toLocaleString()}</div>
          <div className="lbl">DESIGN.md</div>
        </div>
        <div className="stat">
          <div className="num">{formatBytes(data.stats.storage_bytes)}</div>
          <div className="lbl">存储用量</div>
        </div>
      </div>

      <div className="layout-detail">
        <div>
          <section className="panel panel-pad">
            <h2>待办事项</h2>
            {todos.map((t) => (
              <div className="todo-row" key={t.lbl}>
                <span className="num">{t.num}</span>
                <span className="lbl">{t.lbl}</span>
                <Link className="btn btn-outline btn-sm" to={t.to}>
                  前往处理
                </Link>
              </div>
            ))}
          </section>
        </div>

        <aside>
          <section className="panel panel-pad">
            <h2>系统信息</h2>
            <div className="meta-list">
              <div className="row">
                <span className="k">版本</span>
                <span className="v mono">{data.system.version}</span>
              </div>
              <div className="row">
                <span className="k">数据库</span>
                <span className="v">
                  {data.system.database === "mysql" ? "MySQL" : "SQLite"}{" "}
                  <span className="muted" style={{ fontSize: 12 }}>
                    (可在 {data.system.data_dir}/config.toml 查看)
                  </span>
                </span>
              </div>
              <div className="row">
                <span className="k">注册模式</span>
                <span className="v">
                  {REG_MODE_LABELS[data.system.registration_mode] ?? data.system.registration_mode}
                </span>
              </div>
              <div className="row">
                <span className="k">数据目录</span>
                <span className="v mono">{data.system.data_dir}</span>
              </div>
              <div className="row">
                <span className="k">主密钥</span>
                <span className="v">{data.system.secret_created ? "已生成 ✓" : "未生成"}</span>
              </div>
            </div>
          </section>
        </aside>
      </div>
    </>
  );
}
