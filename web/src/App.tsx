import { useEffect, useState } from "react";
import { Navigate, Route, Routes } from "react-router-dom";
import { setupApi } from "./api/setup";
import { AuthProvider } from "./context/AuthContext";
import { Layout } from "./components/Layout";
import { SkillsPage } from "./pages/marketplace/SkillsPage";
import { LoginPage } from "./pages/auth/LoginPage";
import { RegisterPage } from "./pages/auth/RegisterPage";
import { SetupWizard } from "./pages/setup/SetupWizard";

type BootMode = "loading" | "setup" | "normal";

export default function App() {
  const [mode, setMode] = useState<BootMode>("loading");
  const [setupStep, setSetupStep] = useState(1);

  useEffect(() => {
    let cancelled = false;
    setupApi
      .status()
      .then((status) => {
        if (cancelled) return;
        if (status.mode === "setup") {
          setSetupStep(status.step === "admin" ? 2 : 1);
          setMode("setup");
        } else {
          setMode("normal");
        }
      })
      .catch(() => {
        // 安装完成后 /setup/* 返回 404(§5);网络错误时也按正常模式渲染,由各页面自行报错
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
          <Route path="/setup" element={<SetupWizard initialStep={setupStep} />} />
          <Route path="*" element={<Navigate to="/setup" replace />} />
        </Routes>
      ) : (
        <Routes>
          <Route element={<Layout />}>
            <Route path="/" element={<SkillsPage />} />
            <Route path="/login" element={<LoginPage />} />
            <Route path="/register" element={<RegisterPage />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Route>
          <Route path="/setup" element={<Navigate to="/" replace />} />
        </Routes>
      )}
    </AuthProvider>
  );
}
