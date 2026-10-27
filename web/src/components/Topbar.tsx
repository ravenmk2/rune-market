import { ThemeSwitch } from "./ThemeSwitch";
import { Link, NavLink } from "react-router-dom";
import { useAuth } from "../context/AuthContext";
import { useSite } from "../context/SiteContext";
import { UserMenu } from "./UserMenu";

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
            浏览
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
            <UserMenu user={user} />
          ) : (
            <>
              <ThemeSwitch />
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
