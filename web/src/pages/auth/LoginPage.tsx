import { useState } from "react";
import type { FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import { authApi } from "../../api/auth";
import { ApiError } from "../../api/client";
import { useAuth } from "../../context/AuthContext";

export function LoginPage() {
  const { refresh } = useAuth();
  const navigate = useNavigate();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!username || !password) {
      setError("请输入用户名和密码");
      return;
    }
    setError("");
    setSubmitting(true);
    try {
      await authApi.login(username, password);
      await refresh();
      navigate("/", { replace: true });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "登录失败,请稍后重试");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="auth-wrap">
      <div className="auth-card">
        <h1>欢迎回来</h1>
        <p className="sub">登录以发布和管理你的制品</p>

        {error && (
          <div className="alert err" style={{ marginBottom: 18 }}>
            <span>!</span>
            <span>{error}</span>
          </div>
        )}

        <form onSubmit={onSubmit}>
          <div className="field">
            <label htmlFor="f-username">用户名</label>
            <input
              className="input mono"
              id="f-username"
              type="text"
              placeholder="raven"
              autoComplete="username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
            />
          </div>

          <div className="field">
            <label htmlFor="f-password">密码</label>
            <input
              className="input"
              id="f-password"
              type="password"
              placeholder="••••••••"
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </div>

          <button
            className="btn btn-primary"
            type="submit"
            style={{ width: "100%", justifyContent: "center" }}
            disabled={submitting}
          >
            {submitting ? "登录中…" : "登录"}
          </button>
        </form>

        <p className="alt">
          还没有账号?<Link to="/register">注册</Link>
        </p>
      </div>
    </main>
  );
}
