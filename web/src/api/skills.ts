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
  /** 自定义图标 URL(/images/<sha>.<ext>),无图标为 null */
  icon_url: string | null;
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

/** POST /archives 原始响应:report 仅含 checks,metadata 为包内提取信息 */
interface ArchiveUploadRaw {
  sha256: string;
  size: number;
  report: { checks: ValidateCheck[] };
  metadata: ValidateReport["metadata"];
}

/** 压缩包上传结果:sha256 供发布时引用,report 归并为 ValidateReport 复用校验 UI */
export interface ArchiveUpload {
  sha256: string;
  size: number;
  report: ValidateReport;
}

export interface PublishSkillInput {
  /** POST /archives 预上传得到的 sha256 */
  archive: string;
  version: string;
  tags?: string[];
  /** 覆盖包内 SKILL.md 的说明,可空 */
  description?: string;
  /** POST /images 预上传得到的图标 sha256,可空 */
  icon?: string;
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

  /** 发布:JSON body 引用已上传的 archive(多阶段发布第二步,§9) */
  publish: (input: PublishSkillInput) =>
    api
      .post<{ skill: SkillDetail }>("/skills", {
        archive: input.archive,
        version: input.version,
        tags: input.tags && input.tags.length > 0 ? input.tags : undefined,
        description: input.description || undefined,
        icon: input.icon || undefined,
      })
      .then((r) => r.skill),

  mine: () => api.get<ListResult<MySkillItem>>("/mine/skills"),
  /** 更新标签 / 简介 / 说明 / 图标(icon:"" 清除,缺省不变;summary 为制品级,description 作用于当前最新版本) */
  update: (id: string, input: { tags?: string[]; summary?: string; description?: string; icon?: string }) =>
    api.put<void>(`/skills/${id}`, input),
  takedown: (id: string) => api.post<void>(`/skills/${id}/takedown`),
  restore: (id: string) => api.post<void>(`/skills/${id}/restore`),
  remove: (id: string) => api.delete<void>(`/skills/${id}`),
};

/** 压缩包上传(多阶段发布第一步):落库为内容寻址 archive,返回校验报告与包内元数据 */
export const archivesApi = {
  upload: (pkg: Blob) =>
    api.postRaw<ArchiveUploadRaw>("/archives", pkg).then((r) => ({
      sha256: r.sha256,
      size: r.size,
      report: { checks: r.report.checks, metadata: r.metadata } as ValidateReport,
    })),
};

/** 原始压缩包下载地址(浏览器直接导航,走 Content-Disposition) */
export function skillDownloadUrl(ns: string, name: string, ver: string): string {
  return `${API_BASE}/skills/${ns}/${name}/versions/${ver}/download`;
}
