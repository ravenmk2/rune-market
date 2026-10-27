import { createContext, useContext, useEffect } from "react";
import type { ReactNode } from "react";
import type { SiteInfo } from "../api/site";

/** 站点信息在启动时加载，设置保存成功后同步更新。 */
const SiteContext = createContext<SiteInfo>({ mode: "normal" });

const SiteUpdateContext = createContext<(patch: Partial<SiteInfo>) => void>(() => {});
export const useUpdateSite = () => useContext(SiteUpdateContext);

export function SiteProvider({ info, onUpdate, children }: { info: SiteInfo; onUpdate: (patch: Partial<SiteInfo>) => void; children: ReactNode }) {
  useEffect(() => {
    let meta = document.querySelector<HTMLMetaElement>('meta[name="description"]');
    if (!meta) {
      meta = document.createElement("meta");
      meta.name = "description";
      document.head.append(meta);
    }
    meta.content = info.site_description ?? "";
  }, [info.site_description]);
  return <SiteUpdateContext.Provider value={onUpdate}><SiteContext.Provider value={info}>{children}</SiteContext.Provider></SiteUpdateContext.Provider>;
}

export function useSite(): SiteInfo {
  return useContext(SiteContext);
}
