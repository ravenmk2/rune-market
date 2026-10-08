import { api } from "./client";

/** 公共站点信息(品牌展示用,无需登录) */
export interface SiteInfo {
  site_name: string;
  site_description: string;
  site_tagline: string;
  version: string;
}

export const siteApi = {
  info: () => api.get<SiteInfo>("/site"),
};
