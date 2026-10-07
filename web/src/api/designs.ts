import { api, API_BASE } from "./client";
import type { ListResult, Query } from "./client";
import type { Owner, ValidateCheck } from "./skills";

/** DESIGN.md 市场与发布(docs/design.md §8.3 / §8.4 / §10) */

export interface DesignListItem {
  id: string;
  namespace: string;
  name: string;
  summary: string;
  official: boolean;
  status: string;
  tags: string[];
  latest_version: string;
  download_count: number;
  updated_at: string;
  owner: Owner;
  preview_thumb_url: string | null;
}

export interface DesignVersionMeta {
  version: string;
  sha256: string;
  preview_desktop_url: string | null;
  preview_mobile_url: string | null;
  created_at: string;
}

export interface DesignDetail extends DesignListItem {
  latest: DesignVersionMeta;
}

export interface DesignContent {
  content: string;
  sha256: string;
  version: string;
}

/** 弱验证报告(§10:警告不拦截) */
export interface DesignValidateReport {
  checks: ValidateCheck[];
}

export interface PublishDesignInput {
  name: string;
  summary: string;
  version: string;
  tags: string[];
  /** POST /blobs 预上传得到的 sha256,可空 */
  preview_desktop?: string;
  preview_mobile?: string;
}

export const designsApi = {
  list: (query?: Query) => api.get<ListResult<DesignListItem>>("/designs", query),
  detail: (ns: string, name: string) =>
    api.get<{ design: DesignDetail }>(`/designs/${ns}/${name}`).then((r) => r.design),
  versions: (ns: string, name: string) =>
    api.get<{ items: DesignVersionMeta[] }>(`/designs/${ns}/${name}/versions`).then((r) => r.items),
  content: (ns: string, name: string, ver: string) =>
    api.get<DesignContent>(`/designs/${ns}/${name}/versions/${ver}/content`),

  /** 校验:raw body = .md 文本,不落库 */
  validate: (name: string, md: Blob) =>
    api.postRaw<DesignValidateReport>("/designs/validate", md, { name }),
  /** 发布:raw body = .md 文本,元数据走 query */
  publish: (md: Blob, input: PublishDesignInput) =>
    api
      .postRaw<{ design: DesignDetail }>("/designs", md, {
        name: input.name,
        summary: input.summary,
        version: input.version,
        tags: input.tags.join(","),
        preview_desktop: input.preview_desktop || undefined,
        preview_mobile: input.preview_mobile || undefined,
      })
      .then((r) => r.design),

  mine: () => api.get<ListResult<DesignListItem>>("/mine/designs"),
  update: (id: string, input: { summary: string; tags: string[] }) =>
    api.put<void>(`/designs/${id}`, input),
  takedown: (id: string) => api.post<void>(`/designs/${id}/takedown`),
  restore: (id: string) => api.post<void>(`/designs/${id}/restore`),
  remove: (id: string) => api.delete<void>(`/designs/${id}`),
};

/** DESIGN.md 原文下载地址(text/markdown,从 DB 吐) */
export function designDownloadUrl(ns: string, name: string, ver: string): string {
  return `${API_BASE}/designs/${ns}/${name}/versions/${ver}/download`;
}
