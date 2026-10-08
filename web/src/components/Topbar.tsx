import { Link, NavLink } from "react-router-dom";
import { useAuth } from "../context/AuthContext";
import { useSite } from "../context/SiteContext";
import { Avatar } from "./Avatar";

function navClass({ isActive }: { isActive: boolean }) {
  return isActive ? "active" : "";
}

export function Topbar() {
  const { user } = useAuth();
  const siteName = useSite().site_name || "RuneMarket";

  return (
    <header className="topbar">
      <div className="topbar-inner">
        <Link className="brand" to="/">
          <span className="brand-mark">ᚱ</span>
          {siteName}
        </Link>
        <nav className="nav">
          <NavLink to="/" end className={navClass}>
            市场
          </NavLink>
          <NavLink to="/publish" className={navClass}>
            发布
          </NavLink>
          {user?.role === "admin" && (
            <NavLink to="/admin" className={navClass}>
              管理面板
            </NavLink>
          )}
        </nav>
        <div className="topbar-actions">
          {user ? (
            <>
              <Link className="btn btn-outline btn-sm" to="/mine/skills">
                我的发布
              </Link>
              <Link to="/account" title={user.nickname || user.username}>
                <Avatar user={user} />
              </Link>
            </>
          ) : (
            <>
              <Link className="btn btn-outline btn-sm" to="/login">
                登录
              </Link>
              <Link className="btn btn-primary btn-sm" to="/register">
                注册
              </Link>
            </>
          )}
        </div>
      </div>
    </header>
  );
}
