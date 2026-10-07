import { useCallback, useEffect, useState } from "react";
import type { FormEvent } from "react";
import { Link } from "react-router-dom";
import { adminApi } from "../../api/admin";
import type { AdminUser } from "../../api/admin";
import type { ListResult } from "../../api/client";
import { ApiError } from "../../api/client";
import { Pagination } from "../../components/Pagination";
import { formatDate } from "../../utils/format";

const STATUS_FILTERS = [
  { key: "", label: "全部" },
  { key: "active", label: "正常" },
  { key: "pending", label: "待审核" },
  { key: "disabled", label: "已禁用" },
];

const STATUS_LABELS: Record<string, { cls: string; label: string }> = {
  active: { cls: "active", label: "正常" },
  pending: { cls: "pending", label: "待审核" },
  disabled: { cls: "disabled", label: "已禁用" },
};

export function UsersPage() {
  const [input, setInput] = useState("");
  const [q, setQ] = useState("");
  const [status, setStatus] = useState("");
  const [page, setPage] = useState(1);
  const [data, setData] = useState<ListResult<AdminUser> | null>(null);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [tempPassword, setTempPassword] = useState<{ username: string; password: string } | null>(null);
  const [acting, setActing] = useState("");

  const load = useCallback(async () => {
    setError("");
    try {
      setData(
        await adminApi.users({ q: q || undefined, status: status || undefined, page }),
      );
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "加载失败,请稍后重试");
    }
  }, [q, status, page]);

  useEffect(() => {
    void load();
  }, [load]);

  function search(e: FormEvent) {
    e.preventDefault();
    setPage(1);
    setQ(input.trim());
  }

  async function act(user: AdminUser, fn: () => Promise<unknown>) {
    setActing(user.id);
    setError("");
    setNotice("");
    setTempPassword(null);
    try {
      await fn();
      await load();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "操作失败,请稍后重试");
    } finally {
      setActing("");
    }
  }

  const resetPassword = (u: AdminUser) =>
    act(u, async () => {
      const r = await adminApi.resetPassword(u.id);
      setTempPassword({ username: u.username, password: r.temporary_password });
    });

  const removeUser = (u: AdminUser) => {
    if (!window.confirm(`确定删除用户 @${u.username} 吗?其制品将级联下架,不可恢复。`)) return;
    return act(u, () => adminApi.deleteUser(u.id));
  };

  const rejectUser = (u: AdminUser) => {
    if (!window.confirm(`确定拒绝 @${u.username} 的注册吗?账号将被禁用。`)) return;
    return act(u, () => adminApi.disableUser(u.id));
  };

  return (
    <>
      <h1>用户</h1>
      <p className="sub">管理账号、角色与状态</p>

      <form
        style={{ display: "flex", alignItems: "center", gap: 12, marginBottom: 14 }}
        onSubmit={search}
      >
        <input
          className="input"
          type="text"
          placeholder="搜索用户名"
          style={{ width: 280 }}
          value={input}
          onChange={(e) => setInput(e.target.value)}
        />
        <span style={{ marginLeft: "auto" }}></span>
        <Link className="btn btn-primary" to="/admin/users/new">
          新建用户
        </Link>
      </form>

      <div className="filter-row" style={{ margin: "0 0 18px" }}>
        {STATUS_FILTERS.map((f) => (
          <button
            key={f.key}
            className={`chip${status === f.key ? " active" : ""}`}
            onClick={() => {
              setPage(1);
              setStatus(f.key);
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
      {notice && (
        <div className="alert ok" style={{ marginBottom: 18 }}>
          <span>✓</span>
          <span>{notice}</span>
        </div>
      )}
      {tempPassword && (
        <div className="alert ok" style={{ marginBottom: 18 }}>
          <span>✓</span>
          <span>
            已重置 <b>@{tempPassword.username}</b> 的密码,临时密码{" "}
            <code className="mono" style={{ background: "var(--sand)", padding: "1px 6px", borderRadius: 4 }}>
              {tempPassword.password}
            </code>
            (仅展示一次,请立即转交用户)
          </span>
        </div>
      )}

      <section className="panel">
        <table className="table">
          <thead>
            <tr>
              <th>用户</th>
              <th>角色</th>
              <th>注册时间</th>
              <th>状态</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {!data ? (
              <tr>
                <td colSpan={5} className="muted" style={{ textAlign: "center" }}>
                  加载中…
                </td>
              </tr>
            ) : data.items.length === 0 ? (
              <tr>
                <td colSpan={5} className="muted" style={{ textAlign: "center" }}>
                  没有匹配的用户。
                </td>
              </tr>
            ) : (
              data.items.map((u) => {
                const st = STATUS_LABELS[u.status] ?? STATUS_LABELS.active;
                const busy = acting === u.id;
                return (
                  <tr key={u.id}>
                    <td>
                      <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
                        <span className="avatar">
                          {u.has_avatar ? (
                            <img src={`/avatars/${u.id}_32.png`} alt={u.nickname} />
                          ) : (
                            (u.nickname || u.username).charAt(0).toUpperCase()
                          )}
                        </span>
                        <div>
                          <div style={{ fontWeight: 600 }}>{u.nickname || u.username}</div>
                          <div className="muted" style={{ fontSize: 12.5 }}>
                            @{u.username}
                          </div>
                        </div>
                      </div>
                    </td>
                    <td>
                      {u.is_founder && <span className="badge-role founder">创始</span>}{" "}
                      {u.role === "admin" ? (
                        <span className="badge-role admin">管理员</span>
                      ) : (
                        !u.is_founder && <span className="badge-role user">用户</span>
                      )}
                    </td>
                    <td className="muted">{formatDate(u.created_at)}</td>
                    <td>
                      <span className={`status-dot ${st.cls}`}>{st.label}</span>
                    </td>
                    <td style={{ whiteSpace: "nowrap" }}>
                      {u.is_founder ? (
                        <button className="btn btn-outline btn-sm" disabled={busy} onClick={() => resetPassword(u)}>
                          重置密码
                        </button>
                      ) : (
                        <>
                          {u.status === "pending" && (
                            <>
                              <button className="btn btn-outline btn-sm" disabled={busy} onClick={() => act(u, () => adminApi.approveUser(u.id))}>
                                批准
                              </button>{" "}
                              <button className="btn btn-outline btn-sm" disabled={busy} onClick={() => rejectUser(u)}>
                                拒绝
                              </button>{" "}
                            </>
                          )}
                          {u.status === "active" && (
                            <>
                              <button
                                className="btn btn-outline btn-sm"
                                disabled={busy}
                                onClick={() => act(u, () => adminApi.setRole(u.id, u.role === "admin" ? "user" : "admin"))}
                              >
                                {u.role === "admin" ? "取消管理员" : "设为管理员"}
                              </button>{" "}
                              <button className="btn btn-outline btn-sm" disabled={busy} onClick={() => act(u, () => adminApi.disableUser(u.id))}>
                                禁用
                              </button>{" "}
                            </>
                          )}
                          {u.status === "disabled" && (
                            <>
                              <button className="btn btn-outline btn-sm" disabled={busy} onClick={() => act(u, () => adminApi.enableUser(u.id))}>
                                启用
                              </button>{" "}
                            </>
                          )}
                          <button className="btn btn-outline btn-sm" disabled={busy} onClick={() => resetPassword(u)}>
                            重置密码
                          </button>{" "}
                          <button className="btn btn-danger btn-sm" disabled={busy} onClick={() => removeUser(u)}>
                            删除
                          </button>
                        </>
                      )}
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

      <p className="muted" style={{ fontSize: 13, marginTop: 12 }}>
        创始用户由安装向导创建,无法被删除、禁用或降级。
      </p>
    </>
  );
}
