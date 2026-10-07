import { useEffect, useRef, useState } from "react";
import type { FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import { accountApi } from "../../api/account";
import { usersApi } from "../../api/users";
import { ApiError } from "../../api/client";
import { useAuth } from "../../context/AuthContext";
import { Avatar } from "../../components/Avatar";

export function AccountSettingsPage() {
  const { user, loading: authLoading, refresh, logout } = useAuth();
  const navigate = useNavigate();

  const fileRef = useRef<HTMLInputElement>(null);
  const inited = useRef(false);

  const [nickname, setNickname] = useState("");
  const [bio, setBio] = useState("");
  const [profileSaving, setProfileSaving] = useState(false);
  const [profileSaved, setProfileSaved] = useState(false);
  const [avatarBusy, setAvatarBusy] = useState(false);
  const [profileError, setProfileError] = useState("");

  const [curPassword, setCurPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [newPassword2, setNewPassword2] = useState("");
  const [pwBusy, setPwBusy] = useState(false);
  const [pwError, setPwError] = useState("");
  const [pwDone, setPwDone] = useState(false);

  const [dangerError, setDangerError] = useState("");

  useEffect(() => {
    if (!user || inited.current) return;
    inited.current = true;
    setNickname(user.nickname);
    // /auth/me 不含 bio,从公开主页预填
    usersApi
      .profile(user.username)
      .then((r) => setBio(r.user.bio ?? ""))
      .catch(() => undefined);
  }, [user]);

  if (!authLoading && !user) {
    return (
      <main className="container">
        <section className="page-head">
          <h1>账号设置</h1>
        </section>
        <div className="alert warn" style={{ marginTop: 20 }}>
          <span>!</span>
          <span>
            请先<Link to="/login" style={{ fontWeight: 600 }}>登录</Link>。
          </span>
        </div>
      </main>
    );
  }
  if (!user) {
    return (
      <main className="container">
        <p className="muted" style={{ padding: "60px 0", textAlign: "center" }}>
          加载中…
        </p>
      </main>
    );
  }

  const currentUser = user;

  async function uploadAvatar(file: File) {
    if (file.size > 5 * 1024 * 1024) {
      setProfileError("头像文件不能超过 5 MB");
      return;
    }
    setAvatarBusy(true);
    setProfileError("");
    try {
      await accountApi.uploadAvatar(file);
      // updated_at 变更 → Avatar 的 ?v= 随之失效刷新
      await refresh();
    } catch (e) {
      setProfileError(e instanceof ApiError ? e.message : "头像上传失败,请重试");
    } finally {
      setAvatarBusy(false);
    }
  }

  async function removeAvatar() {
    if (!window.confirm("确定移除当前头像吗?将回落为字母占位。")) return;
    setAvatarBusy(true);
    setProfileError("");
    try {
      await accountApi.removeAvatar();
      await refresh();
    } catch (e) {
      setProfileError(e instanceof ApiError ? e.message : "移除失败,请重试");
    } finally {
      setAvatarBusy(false);
    }
  }

  async function saveProfile(e: FormEvent) {
    e.preventDefault();
    if (!nickname.trim()) {
      setProfileError("请输入昵称");
      return;
    }
    setProfileSaving(true);
    setProfileSaved(false);
    setProfileError("");
    try {
      await accountApi.update({ nickname: nickname.trim(), bio: bio.trim() });
      await refresh();
      setProfileSaved(true);
    } catch (err) {
      setProfileError(err instanceof ApiError ? err.message : "保存失败,请稍后重试");
    } finally {
      setProfileSaving(false);
    }
  }

  async function changePassword(e: FormEvent) {
    e.preventDefault();
    if (newPassword.length < 8) {
      setPwError("新密码至少 8 位");
      return;
    }
    if (newPassword !== newPassword2) {
      setPwError("两次输入的新密码不一致");
      return;
    }
    setPwBusy(true);
    setPwError("");
    try {
      await accountApi.changePassword(curPassword, newPassword);
      // 后端已作废旧会话,刷新会话状态(将变为未登录)
      await refresh();
      setPwDone(true);
    } catch (err) {
      setPwError(err instanceof ApiError ? err.message : "修改失败,请稍后重试");
    } finally {
      setPwBusy(false);
    }
  }

  async function deleteAccount() {
    const input = window.prompt(
      `注销账号将删除账号及全部制品,不可恢复。请输入用户名 "${currentUser.username}" 确认:`,
    );
    if (input === null) return;
    if (input.trim() !== currentUser.username) {
      setDangerError("输入的用户名不匹配,已取消注销。");
      return;
    }
    setDangerError("");
    try {
      await accountApi.deleteAccount();
    } catch (err) {
      setDangerError(err instanceof ApiError ? err.message : "注销失败,请稍后重试");
      return;
    }
    // 会话随账号删除失效,清理本地状态并回首页
    await logout();
    navigate("/");
  }

  return (
    <main className="container">
      <section className="page-head">
        <h1>账号设置</h1>
      </section>

      <div style={{ maxWidth: 640, paddingBottom: 20 }}>
        <div className="panel panel-pad" style={{ marginTop: 18 }}>
          <h2>资料</h2>
          {profileError && (
            <div className="alert err" style={{ marginBottom: 18 }}>
              <span>!</span>
              <span>{profileError}</span>
            </div>
          )}
          {profileSaved && (
            <div className="alert ok" style={{ marginBottom: 18 }}>
              <span>✓</span>
              <span>资料已保存。</span>
            </div>
          )}
          <form onSubmit={saveProfile}>
            <div className="field">
              <label>头像</label>
              <div style={{ display: "flex", alignItems: "center", gap: 14 }}>
                <Avatar user={user} size={64} />
                <button
                  className="btn btn-outline btn-sm"
                  type="button"
                  disabled={avatarBusy}
                  onClick={() => fileRef.current?.click()}
                >
                  {avatarBusy ? "上传中…" : "上传头像"}
                </button>
                {user.has_avatar && (
                  <button
                    className="btn btn-outline btn-sm"
                    type="button"
                    disabled={avatarBusy}
                    onClick={removeAvatar}
                  >
                    移除
                  </button>
                )}
                <input
                  ref={fileRef}
                  type="file"
                  accept="image/png,image/jpeg"
                  style={{ display: "none" }}
                  onChange={(e) => {
                    const f = e.target.files?.[0];
                    if (f) void uploadAvatar(f);
                    e.target.value = "";
                  }}
                />
              </div>
              <div className="hint">
                支持 PNG / JPG,最大 5 MB;上传后自动裁剪为正方形并生成 32 / 64 / 128px 缩略图。
              </div>
            </div>
            <div className="field">
              <label htmlFor="f-nickname">昵称</label>
              <input
                className="input"
                id="f-nickname"
                type="text"
                value={nickname}
                onChange={(e) => setNickname(e.target.value)}
              />
              <div className="hint">展示在个人主页与制品页面</div>
            </div>
            <div className="field">
              <label htmlFor="f-bio">个人简介</label>
              <textarea
                className="textarea"
                id="f-bio"
                placeholder="一句话介绍自己"
                value={bio}
                onChange={(e) => setBio(e.target.value)}
              />
            </div>
            <div className="field">
              <label htmlFor="f-username">用户名</label>
              <input className="input mono" id="f-username" type="text" value={user.username} disabled />
              <div className="hint">用户名即命名空间,不可修改</div>
            </div>
            <button className="btn btn-primary" type="submit" disabled={profileSaving}>
              {profileSaving ? "保存中…" : "保存"}
            </button>
          </form>
        </div>

        <div className="panel panel-pad">
          <h2>修改密码</h2>
          {pwDone ? (
            <>
              <div className="alert ok" style={{ marginBottom: 16 }}>
                <span>✓</span>
                <span>
                  <b>密码已更新。</b>出于安全考虑,所有旧会话已失效,请使用新密码重新登录。
                </span>
              </div>
              <Link className="btn btn-primary" to="/login">
                前往登录
              </Link>
            </>
          ) : (
            <form onSubmit={changePassword}>
              {pwError && (
                <div className="alert err" style={{ marginBottom: 18 }}>
                  <span>!</span>
                  <span>{pwError}</span>
                </div>
              )}
              <div className="field">
                <label htmlFor="f-cur">当前密码</label>
                <input
                  className="input"
                  id="f-cur"
                  type="password"
                  placeholder="••••••••"
                  autoComplete="current-password"
                  value={curPassword}
                  onChange={(e) => setCurPassword(e.target.value)}
                />
              </div>
              <div className="field-2col">
                <div className="field">
                  <label htmlFor="f-new">新密码</label>
                  <input
                    className="input"
                    id="f-new"
                    type="password"
                    placeholder="至少 8 位"
                    autoComplete="new-password"
                    value={newPassword}
                    onChange={(e) => setNewPassword(e.target.value)}
                  />
                </div>
                <div className="field">
                  <label htmlFor="f-new2">确认新密码</label>
                  <input
                    className="input"
                    id="f-new2"
                    type="password"
                    placeholder="再次输入新密码"
                    autoComplete="new-password"
                    value={newPassword2}
                    onChange={(e) => setNewPassword2(e.target.value)}
                  />
                </div>
              </div>
              <button className="btn btn-primary" type="submit" disabled={pwBusy}>
                {pwBusy ? "更新中…" : "更新密码"}
              </button>
            </form>
          )}
        </div>

        <div className="panel panel-pad danger">
          <h2>危险区</h2>
          {dangerError && (
            <div className="alert err" style={{ marginBottom: 14 }}>
              <span>!</span>
              <span>{dangerError}</span>
            </div>
          )}
          <div className="danger-row">
            <div>
              <div className="dr-title">注销账号</div>
              <div className="dr-desc">
                {user.is_founder
                  ? "创始用户由安装向导创建,不可注销"
                  : "删除账号及全部制品,不可恢复"}
              </div>
            </div>
            {!user.is_founder && (
              <button className="btn btn-danger btn-sm" type="button" onClick={deleteAccount}>
                注销账号
              </button>
            )}
          </div>
        </div>
      </div>
    </main>
  );
}
