import { api } from "./client";

/** 公共站点信息(品牌展示用,无需登录);安装模式下仅含 mode/step */
export interface SiteInfo {
  mode: "normal" | "setup";
  step?: string;
  site_name?: string;
  site_description?: string;
  site_tagline?: string;
  version?: string;
}

export const siteApi = {
  info: () => api.get<SiteInfo>("/site"),
};
