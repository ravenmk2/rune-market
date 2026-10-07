import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";
import { useNavigate } from "react-router-dom";
import { authApi } from "../api/auth";
import type { User } from "../api/auth";
import { setUnauthorizedHandler } from "../api/client";

interface AuthContextValue {
  /** 当前登录用户;未登录为 null */
  user: User | null;
  /** 首次会话探测进行中 */
  loading: boolean;
  /** 重新拉取 /auth/me(登录后调用) */
  refresh: () => Promise<void>;
  logout: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

/** 401 后不跳转登录页的公开路径 */
const PUBLIC_PATHS = ["/", "/login", "/register", "/setup"];

export function AuthProvider({ children, enabled = true }: { children: ReactNode; enabled?: boolean }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(enabled);
  const navigate = useNavigate();

  const refresh = useCallback(async () => {
    try {
      setUser(await authApi.me());
    } catch {
      setUser(null);
    }
  }, []);

  const logout = useCallback(async () => {
    try {
      await authApi.logout();
    } finally {
      setUser(null);
      navigate("/");
    }
  }, [navigate]);

  useEffect(() => {
    setUnauthorizedHandler(() => {
      setUser(null);
      if (!PUBLIC_PATHS.includes(window.location.pathname)) navigate("/login");
    });
    return () => setUnauthorizedHandler(null);
  }, [navigate]);

  useEffect(() => {
    // setup 模式下无会话,不探测(enabled=false)
    if (!enabled) return;
    void refresh().finally(() => setLoading(false));
  }, [enabled, refresh]);

  const value = useMemo(() => ({ user, loading, refresh, logout }), [user, loading, refresh, logout]);

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth 必须在 AuthProvider 内使用");
  return ctx;
}
