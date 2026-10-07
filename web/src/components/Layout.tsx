import { Outlet } from "react-router-dom";
import { Topbar } from "./Topbar";
import { Footer } from "./Footer";

/** 基础布局:顶栏 + 内容 + 页脚 */
export function Layout() {
  return (
    <>
      <Topbar />
      <Outlet />
      <Footer />
    </>
  );
}
