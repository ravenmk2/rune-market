import { useState } from "react";
import type { FormEvent } from "react";
import { Link } from "react-router-dom";
import { authApi } from "../../api/auth";
import { ApiError } from "../../api/client";

/** 用户名规则(docs/design.md §7):小写字母/数字/连字符,≤64 */
const USERNAME_RE = /^[a-z0-9](-?[a-z0-9])*$/;

export function RegisterPage() {
  const [username, setUsername] = useState("");
  const [nickname, setNickname] = useState("");
  const [password, setPassword] = useState("");
  const [password2, setPassword2] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  /** 注册完成态:pending = 待管理员审批 */
  const [done, setDone] = useState<"active" | "pending" | null>(null);

  async function onSubmit(e: FormEvent) {
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
      setError("密码至少 8 位");
      return;
    }
    if (password !== password2) {
      setError("两次输入的密码不一致");
      return;
    }
    setError("");
    setSubmitting(true);
    try {
      const result = await authApi.register({ username, nickname: nickname.trim(), password });
      setDone(result.status === "pending" ? "pending" : "active");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "注册失败,请稍后重试");
    } finally {
      setSubmitting(false);
    }
  }

  if (done) {
    return (
      <main className="auth-wrap">
        <div className="auth-card">
          <h1>{done === "pending" ? "注册已提交" : "注册成功"}</h1>
          <p className="sub">
            {done === "pending" ? "你的账号需要管理员批准后才能使用" : "现在可以使用新账号登录了"}
          </p>
          {done === "pending" && (
            <div className="alert warn" style={{ marginBottom: 20 }}>
              <span>!</span>
              <span>
                <b>账号待审批。</b>管理员批准后,你即可登录 RuneMarket。
              </span>
            </div>
          )}
          <Link
            className="btn btn-primary"
            to="/login"
            style={{ width: "100%", justifyContent: "center" }}
          >
            前往登录
          </Link>
        </div>
      </main>
    );
  }

  return (
    <main className="auth-wrap">
      <div className="auth-card">
        <h1>创建账号</h1>
        <p className="sub">发布你的第一个 Skill 或 DESIGN.md</p>

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
            <div className="hint">小写字母、数字、连字符,将成为你的命名空间</div>
          </div>

          <div className="field">
            <label htmlFor="f-nickname">昵称</label>
            <input
              className="input"
              id="f-nickname"
              type="text"
              placeholder="Raven"
              value={nickname}
              onChange={(e) => setNickname(e.target.value)}
            />
            <div className="hint">展示在个人主页与制品页面,可随时修改</div>
          </div>

          <div className="field">
            <label htmlFor="f-password">密码</label>
            <input
              className="input"
              id="f-password"
              type="password"
              placeholder="至少 8 位"
              autoComplete="new-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </div>

          <div className="field">
            <label htmlFor="f-password2">确认密码</label>
            <input
              className="input"
              id="f-password2"
              type="password"
              placeholder="再次输入密码"
              autoComplete="new-password"
              value={password2}
              onChange={(e) => setPassword2(e.target.value)}
            />
          </div>

          <button
            className="btn btn-primary"
            type="submit"
            style={{ width: "100%", justifyContent: "center" }}
            disabled={submitting}
          >
            {submitting ? "注册中…" : "注册"}
          </button>
        </form>

        <p className="alt">
          已有账号?<Link to="/login">登录</Link>
        </p>
      </div>
    </main>
  );
}
