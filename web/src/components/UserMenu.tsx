import { useEffect, useId, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useAuth } from "../context/AuthContext";
import { Link, useLocation } from "react-router-dom";
import type { User } from "../api/auth";
import { Avatar } from "./Avatar";
import { ThemeOptions } from "./ThemeSwitch";
import "./UserMenu.css";

export function UserMenu({ user }: { user: User }) {
  const [open, setOpen] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const dialog = useRef<HTMLDialogElement>(null);
  const cancel = useRef<HTMLButtonElement>(null);
  const submitting = useRef(false);
  const { logout } = useAuth();
  const root = useRef<HTMLDivElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);
  const panel = useRef<HTMLDivElement>(null);
  const id = useId();
  const location = useLocation();
  useEffect(() => {
    if (confirming) {
      dialog.current?.showModal();
      cancel.current?.focus();
    }
  }, [confirming]);
  function closeConfirmation() {
    if (submitting.current) return;
    setConfirming(false);
    trigger.current?.focus();
  }
  async function confirmLogout() {
    if (submitting.current) return;
    submitting.current = true;
    setBusy(true);
    setError("");
    try {
      await logout();
    } catch {
      setError("退出失败，请重试");
    } finally {
      submitting.current = false;
      setBusy(false);
    }
  }
  useEffect(() => { setOpen(false); }, [location]);
  useEffect(() => {
    if (!open) return;
    const outside = (event: PointerEvent) => {
      if (!root.current?.contains(event.target as Node)) setOpen(false);
    };
    document.addEventListener("pointerdown", outside);
    return () => document.removeEventListener("pointerdown", outside);
  }, [open]);
  return <div className="user-menu" ref={root}
    onBlur={event => { if (!event.currentTarget.contains(event.relatedTarget as Node)) setOpen(false); }}
    onKeyDown={event => {
      if (event.key === "Escape") { event.preventDefault(); setOpen(false); trigger.current?.focus(); }
      if (open && ["ArrowDown", "ArrowUp", "Home", "End"].includes(event.key)) {
        event.preventDefault();
        const controls = Array.from(panel.current?.querySelectorAll<HTMLElement>("a, button") ?? []);
        const current = controls.indexOf(document.activeElement as HTMLElement);
        const index = event.key === "Home" ? 0 : event.key === "End" ? controls.length - 1 : (current + (event.key === "ArrowDown" ? 1 : -1) + controls.length) % controls.length;
        controls[index]?.focus();
      }
    }}>
    <button type="button" className="user-menu-trigger" ref={trigger} aria-label={`用户菜单：${user.nickname || user.username}`} title="用户菜单" aria-expanded={open} aria-controls={open ? id : undefined} onClick={() => setOpen(!open)}>
      <Avatar user={user} />
    </button>
    {open && <div className="user-menu-panel" id={id} ref={panel}>
      <div className="user-menu-identity"><strong>{user.nickname || user.username}</strong><span>@{user.username}</span></div>
      <nav aria-label="用户操作" className="user-menu-links">
        <Link to="/mine/skills"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinejoin="round" aria-hidden="true"><path d="m12 3 9 5-9 5-9-5 9-5Zm-9 5v9l9 5 9-5V8M12 13v9M7.5 5.5l9 5"/></svg>我的发布</Link>
        <Link to="/account"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" aria-hidden="true"><circle cx="12" cy="8" r="4"/><path d="M4 21v-2a8 8 0 0 1 16 0v2"/></svg>账号设置</Link>
      </nav>
      <div className="user-menu-theme"><div className="user-menu-label">外观</div><ThemeOptions /></div>
      <div className="user-menu-logout">
        <button type="button" onClick={() => { setOpen(false); setError(""); setConfirming(true); }}>
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d="M9 4H4v16h5M9 12h12m-4-4 4 4-4 4" /></svg>
          退出登录
        </button>
      </div>
    </div>}
    {confirming && createPortal(<dialog ref={dialog} className="dialog logout-dialog" aria-labelledby={`${id}-logout-title`} aria-describedby={`${id}-logout-description`} aria-busy={busy}
      onCancel={event => { event.preventDefault(); closeConfirmation(); }}
      onClick={event => { event.stopPropagation(); if (event.target === event.currentTarget) { const rect = event.currentTarget.getBoundingClientRect(); if (event.clientX < rect.left || event.clientX > rect.right || event.clientY < rect.top || event.clientY > rect.bottom) closeConfirmation(); } }}
      onKeyDown={event => event.stopPropagation()}>
      <h2 id={`${id}-logout-title`}>退出登录</h2>
      <p id={`${id}-logout-description`}>确定退出当前账号吗？</p>
      {error && <p className="logout-error" role="alert">{error}</p>}
      <div className="dialog-actions">
        <button ref={cancel} type="button" className="btn btn-outline" disabled={busy} onClick={closeConfirmation}>取消</button>
        <button type="button" className="btn btn-primary" disabled={busy} onClick={() => void confirmLogout()}>{busy ? "退出中…" : "退出登录"}</button>
      </div>
    </dialog>, document.body)}
  </div>;
}
