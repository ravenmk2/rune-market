import { useState } from "react";
import type { FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import { adminApi } from "../../api/admin";
import { ApiError } from "../../api/client";

const USERNAME_RE = /^[a-z0-9](-?[a-z0-9])*$/;

export function UserNewPage() {
  const navigate = useNavigate();
  const [username, setUsername] = useState("");
  const [nickname, setNickname] = useState("");
  const [password, setPassword] = useState("");
  const [role, setRole] = useState<"user" | "admin">("user");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!USERNAME_RE.test(username) || username.length > 64) {
      setError("用户名需为小写字母、数字或连字符,且不能以连字符开头/结尾(≤64 字符)");
      return;
    }
    if (!nickname.trim()) {
      setError("请输入昵称");
      return;
    }
    if (password.length < 8) {
      setError("初始密码至少 8 位");
      return;
    }
    setError("");
    setSubmitting(true);
    try {
      await adminApi.createUser({ username, nickname: nickname.trim(), password, role });
      navigate("/admin/users");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "创建失败,请稍后重试");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <>
      <div className="crumbs" style={{ padding: "0 0 18px" }}>
        <Link to="/admin">管理面板</Link> / <Link to="/admin/users">用户</Link> / 新建
      </div>
      <h1>新建用户</h1>
      <p className="sub">注册策略为关闭时,由此创建账号。</p>

      <form onSubmit={submit}>
        <section className="panel panel-pad" style={{ maxWidth: 640 }}>
          {error && (
            <div className="alert err" style={{ marginBottom: 18 }}>
              <span>!</span>
              <span>{error}</span>
            </div>
          )}
          <div className="field">
            <label htmlFor="f-username">用户名</label>
            <input
              className="input mono"
              id="f-username"
              type="text"
              placeholder="例如 littlefox"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
            />
            <div className="hint">小写字母/数字/连字符,将成为命名空间,不可修改</div>
          </div>
          <div className="field">
            <label htmlFor="f-nickname">昵称</label>
            <input
              className="input"
              id="f-nickname"
              type="text"
              placeholder="例如 小狐狸"
              value={nickname}
              onChange={(e) => setNickname(e.target.value)}
            />
            <div className="hint">展示名,用户可自行修改</div>
          </div>
          <div className="field">
            <label htmlFor="f-password">初始密码</label>
            <input
              className="input"
              id="f-password"
              type="password"
              placeholder="••••••••"
              autoComplete="new-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
            <div className="hint">首次登录后应提醒用户修改</div>
          </div>
          <div className="field" style={{ marginBottom: 0 }}>
            <label>角色</label>
            <div className="radio-cards">
              <label className={`radio-card${role === "user" ? " selected" : ""}`}>
                <input
                  type="radio"
                  name="role"
                  checked={role === "user"}
                  onChange={() => setRole("user")}
                />
                <span>
                  <span className="rc-title">普通用户</span>
                  <div className="rc-desc">可发布与管理自己的制品</div>
                </span>
              </label>
              <label className={`radio-card${role === "admin" ? " selected" : ""}`}>
                <input
                  type="radio"
                  name="role"
                  checked={role === "admin"}
                  onChange={() => setRole("admin")}
                />
                <span>
                  <span className="rc-title">管理员</span>
                  <div className="rc-desc">可访问管理面板</div>
                </span>
              </label>
            </div>
          </div>
        </section>

        <div style={{ marginTop: 22 }}>
          <button className="btn btn-primary" type="submit" disabled={submitting}>
            {submitting ? "创建中…" : "创建用户"}
          </button>
        </div>
      </form>
    </>
  );
}
