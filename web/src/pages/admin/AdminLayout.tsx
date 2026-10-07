import { Link, Navigate, NavLink, Outlet } from "react-router-dom";
import { useAuth } from "../../context/AuthContext";
import { Avatar } from "../../components/Avatar";

function sideClass({ isActive }: { isActive: boolean }) {
  return isActive ? "active" : "";
}

/** 管理面板布局:通栏顶栏 + 侧栏 + 居中限宽内容区;仅 admin 可达(后端仍有强制校验) */
export function AdminLayout() {
  const { user, loading } = useAuth();

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
            <NavLink to="/">市场</NavLink>
            <NavLink to="/admin" className={sideClass}>
              管理面板
            </NavLink>
          </nav>
          <div className="topbar-actions">
            <Link className="btn btn-outline btn-sm" to="/">
              返回市场
            </Link>
            <Avatar user={user} />
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

        <main className="admin-main">
          <Outlet />
        </main>
      </div>
    </>
  );
}
