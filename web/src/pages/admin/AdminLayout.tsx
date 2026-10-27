import { UserMenu } from "../../components/UserMenu";
import { Link, Navigate, NavLink, Outlet, useLocation } from "react-router-dom";
import { useAuth } from "../../context/AuthContext";

function sideClass({ isActive }: { isActive: boolean }) {
  return isActive ? "active" : "";
}

/** 管理面板布局:概览与列表自适应宽度，表单居中限宽;仅 admin 可达。 */
export function AdminLayout() {
  const { user, loading } = useAuth();
  const { pathname } = useLocation();
  const fluid = ["/admin", "/admin/users", "/admin/skills", "/admin/designs"].includes(pathname.replace(/\/+$/, ""));

  if (loading) {
    return (
      <div className="boot-splash">
        <div className="setup-brand" style={{ marginBottom: 0 }}>
          <span className="brand-mark">ᚱ</span>RuneMarket
        </div>
      </div>
    );
  }
  if (!user || user.role !== "admin") return <Navigate to="/" replace />;

  return (
    <>
      <header className="topbar">
        <div className="topbar-inner fluid">
          <Link className="brand" to="/">
            <span className="brand-mark">ᚱ</span>RuneMarket
          </Link>
          <nav className="nav">
            <NavLink to="/">浏览</NavLink>
            <NavLink to="/admin" className={sideClass}>
              管理面板
            </NavLink>
          </nav>
          <div className="topbar-actions">
            <Link className="btn btn-outline btn-sm" to="/">
              返回商店
            </Link>
            <UserMenu user={user} />
          </div>
        </div>
      </header>

      <div className="admin-shell">
        <aside className="admin-side">
          <div className="side-title">管理面板</div>
          <NavLink to="/admin" end className={sideClass}>
            概览
          </NavLink>
          <NavLink to="/admin/users" className={sideClass}>
            用户
          </NavLink>
          <NavLink to="/admin/skills" className={sideClass}>
            Skills
          </NavLink>
          <NavLink to="/admin/designs" className={sideClass}>
            DESIGN.md
          </NavLink>
          <NavLink to="/admin/settings" className={sideClass}>
            系统设置
          </NavLink>
        </aside>

        <main className={`admin-main${fluid ? " admin-main-fluid" : ""}`}>
          <Outlet />
        </main>
      </div>
    </>
  );
}
