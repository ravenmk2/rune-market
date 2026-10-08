import { createContext, useContext } from "react";
import type { ReactNode } from "react";
import type { SiteInfo } from "../api/site";

/** 站点信息在 App 启动时拉取一次,全站经 context 共享(§8.3 /site) */
const SiteContext = createContext<SiteInfo>({ mode: "normal" });

export function SiteProvider({ info, children }: { info: SiteInfo; children: ReactNode }) {
  return <SiteContext.Provider value={info}>{children}</SiteContext.Provider>;
}

export function useSite(): SiteInfo {
  return useContext(SiteContext);
}
