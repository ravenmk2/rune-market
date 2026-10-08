import { api, API_BASE } from "./client";
import type { ListResult, Query } from "./client";

/** Skill 市场与发布(docs/design.md §8.3 / §8.4 / §9) */

export interface Owner {
  username: string;
  nickname: string;
}

export interface Permission {
  tool: string;
  risk: "warn" | "ok";
  note: string;
}

export type SkillStatus = "published" | "pending" | "taken_down" | string;

export interface SkillListItem {
  namespace: string;
  name: string;
  summary: string;
  official: boolean;
  status: SkillStatus;
  tags: string[];
  latest_version: string;
  download_count: number;
  updated_at: string;
  owner: Owner;
  /** 列表契约未含,后端若返回则展示 */
  harnesses?: string[];
}

export interface VersionMeta {
  version: string;
  description: string;
  license: string;
  compatibility: string;
  author: string;
  harnesses: string[];
  permissions: Permission[];
  file_count: number;
  sha256: string;
  size: number;
  filename: string;
  created_at: string;
}

export interface SkillDetail extends SkillListItem {
  latest: VersionMeta;
}

export interface ValidateCheck {
  level: "ok" | "warn" | "error";
  title: string;
  detail?: string;
}

export interface ValidateReport {
  checks: ValidateCheck[];
  metadata: {
    name: string;
    description: string;
    license: string;
    compatibility: string;
    author: string;
    harnesses: string[];
    permissions: Permission[];
    file_count: number;
    sha256: string;
    size: number;
  };
}

export interface FileEntry {
  path: string;
  size: number;
}

export interface FileContent {
  path: string;
  size: number;
  content_type: "text" | "binary";
  content: string;
}

/** 我的制品列表项:含制品 id 供下架/编辑/删除操作 */
export interface MySkillItem extends SkillListItem {
  id: string;
}

export const skillsApi = {
  list: (query?: Query) => api.get<ListResult<SkillListItem>>("/skills", query),
  detail: (ns: string, name: string) =>
    api.get<{ skill: SkillDetail }>(`/skills/${ns}/${name}`).then((r) => r.skill),
  versions: (ns: string, name: string) =>
    api.get<{ items: VersionMeta[] }>(`/skills/${ns}/${name}/versions`).then((r) => r.items),
  files: (ns: string, name: string, ver: string) =>
    api
      .get<{ files: FileEntry[] }>(`/skills/${ns}/${name}/versions/${ver}/files`)
      .then((r) => r.files),
  file: (ns: string, name: string, ver: string, path: string) =>
    api.get<FileContent>(`/skills/${ns}/${name}/versions/${ver}/file`, { path }),

  /** 校验:raw body = 压缩包,不落库(§9) */
  validate: (pkg: Blob) => api.postRaw<ValidateReport>("/skills/validate", pkg),
  /** 发布:raw body = 压缩包,元数据走 query(version、逗号分隔 tags、可选 description 覆盖包内说明) */
  publish: (pkg: Blob, version: string, tags: string[], description?: string) =>
    api
      .postRaw<{ skill: SkillDetail }>("/skills", pkg, {
        version,
        tags: tags.join(","),
        description: description || undefined,
      })
      .then((r) => r.skill),

  mine: () => api.get<ListResult<MySkillItem>>("/mine/skills"),
  /** 更新标签 / 简介 / 说明(summary 为制品级字段,description 作用于当前最新版本) */
  update: (id: string, input: { tags?: string[]; summary?: string; description?: string }) =>
    api.put<void>(`/skills/${id}`, input),
  takedown: (id: string) => api.post<void>(`/skills/${id}/takedown`),
  restore: (id: string) => api.post<void>(`/skills/${id}/restore`),
  remove: (id: string) => api.delete<void>(`/skills/${id}`),
};

/** 原始压缩包下载地址(浏览器直接导航,走 Content-Disposition) */
export function skillDownloadUrl(ns: string, name: string, ver: string): string {
  return `${API_BASE}/skills/${ns}/${name}/versions/${ver}/download`;
}
