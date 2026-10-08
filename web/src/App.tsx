import { useEffect, useState } from "react";
import { Navigate, Route, Routes } from "react-router-dom";
import { siteApi } from "./api/site";
import type { SiteInfo } from "./api/site";
import { AuthProvider } from "./context/AuthContext";
import { SiteProvider } from "./context/SiteContext";
import { Layout } from "./components/Layout";
import { SkillsPage } from "./pages/marketplace/SkillsPage";
import { DesignsPage } from "./pages/marketplace/DesignsPage";
import { SkillDetailPage } from "./pages/skill/SkillDetailPage";
import { DesignDetailPage } from "./pages/design/DesignDetailPage";
import { PublishSkillPage } from "./pages/publish/PublishSkillPage";
import { PublishDesignPage } from "./pages/publish/PublishDesignPage";
import { MySkillsPage } from "./pages/mine/MySkillsPage";
import { MyDesignsPage } from "./pages/mine/MyDesignsPage";
import { EditSkillPage } from "./pages/mine/EditSkillPage";
import { EditDesignPage } from "./pages/mine/EditDesignPage";
import { AdminLayout } from "./pages/admin/AdminLayout";
import { OverviewPage } from "./pages/admin/OverviewPage";
import { UsersPage } from "./pages/admin/UsersPage";
import { UserNewPage } from "./pages/admin/UserNewPage";
import { AdminSkillsPage } from "./pages/admin/AdminSkillsPage";
import { AdminDesignsPage } from "./pages/admin/AdminDesignsPage";
import { SettingsPage } from "./pages/admin/SettingsPage";
import { AccountSettingsPage } from "./pages/account/AccountSettingsPage";
import { UserProfilePage } from "./pages/user/UserProfilePage";
import { LoginPage } from "./pages/auth/LoginPage";
import { RegisterPage } from "./pages/auth/RegisterPage";
import { SetupWizard } from "./pages/setup/SetupWizard";

type BootMode = "loading" | "setup" | "normal";

export default function App() {
  const [mode, setMode] = useState<BootMode>("loading");
  const [site, setSite] = useState<SiteInfo>({ mode: "normal" });

  useEffect(() => {
    let cancelled = false;
    // /site 在两种模式下都应答(§5):安装模式返回 mode/step,正常模式返回站点信息
    siteApi
      .info()
      .then((info) => {
        if (cancelled) return;
        if (info.mode === "setup") {
          setMode("setup");
        } else {
          setSite(info);
          setMode("normal");
        }
      })
      .catch(() => {
        // 网络错误时按正常模式渲染,由各页面自行报错
        if (!cancelled) setMode("normal");
      });
    return () => {
      cancelled = true;
    };
  }, []);

  if (mode === "loading") {
    return (
      <div className="boot-splash">
        <div className="setup-brand" style={{ marginBottom: 0 }}>
          <span className="brand-mark">ᚱ</span>RuneMarket
        </div>
      </div>
    );
  }

  return (
    <AuthProvider enabled={mode === "normal"}>
      {mode === "setup" ? (
        <Routes>
          <Route path="/setup" element={<SetupWizard initialStep={site.step === "admin" ? 2 : 1} />} />
          <Route path="*" element={<Navigate to="/setup" replace />} />
        </Routes>
      ) : (
        <SiteProvider info={site}>
          <Routes>
          <Route element={<Layout />}>
            <Route path="/" element={<SkillsPage />} />
            <Route path="/designs" element={<DesignsPage />} />
            <Route path="/s/:ns/:name/*" element={<SkillDetailPage />} />
            <Route path="/d/:ns/:name/*" element={<DesignDetailPage />} />
            <Route path="/publish" element={<PublishSkillPage />} />
            <Route path="/publish/design" element={<PublishDesignPage />} />
            <Route path="/mine/skills" element={<MySkillsPage />} />
            <Route path="/mine/skills/:id/edit" element={<EditSkillPage />} />
            <Route path="/mine/designs" element={<MyDesignsPage />} />
            <Route path="/mine/designs/:id/edit" element={<EditDesignPage />} />
            <Route path="/account" element={<AccountSettingsPage />} />
            <Route path="/u/:username" element={<UserProfilePage />} />
            <Route path="/login" element={<LoginPage />} />
            <Route path="/register" element={<RegisterPage />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Route>
          <Route path="/admin" element={<AdminLayout />}>
            <Route index element={<OverviewPage />} />
            <Route path="users" element={<UsersPage />} />
            <Route path="users/new" element={<UserNewPage />} />
            <Route path="skills" element={<AdminSkillsPage />} />
            <Route path="designs" element={<AdminDesignsPage />} />
            <Route path="settings" element={<SettingsPage />} />
          </Route>
          <Route path="/setup" element={<Navigate to="/" replace />} />
          </Routes>
        </SiteProvider>
      )}
    </AuthProvider>
  );
}
