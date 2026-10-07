import { api } from "./client";
import type { ListResult, Query } from "./client";
import type { SkillListItem } from "./skills";
import type { DesignListItem } from "./designs";

/** 管理面板(docs/design.md §8.5,requireAdmin) */

export interface AdminOverview {
  stats: {
    users: number;
    skills: number;
    designs: number;
    storage_bytes: number;
  };
  todos: {
    pending_users: number;
    pending_skills: number;
    pending_designs: number;
  };
  system: {
    version: string;
    database: string;
    registration_mode: "open" | "approval" | "closed" | string;
    data_dir: string;
    secret_created: boolean;
  };
}

export type AdminUserStatus = "active" | "pending" | "disabled";

export interface AdminUser {
  id: string;
  username: string;
  nickname: string;
  role: "admin" | "user";
  is_founder: boolean;
  status: AdminUserStatus;
  has_avatar: boolean;
  created_at: string;
}

/** 制品状态:published | pending | taken_down */
export type ArtifactStatus = "published" | "pending" | "taken_down" | string;

export interface AdminSkillItem extends SkillListItem {
  id: string;
}

export type AdminDesignItem = DesignListItem;

/** 系统设置键(§6.3) */
export interface Settings {
  site_name?: string;
  site_description?: string;
  page_size?: number;
  registration_mode?: "open" | "approval" | "closed";
  artifact_review?: "none" | "required";
  upload_max_mb?: number;
  anonymous_browse?: boolean;
  anonymous_download?: boolean;
}

export const adminApi = {
  overview: () => api.get<AdminOverview>("/admin/overview"),

  users: (query?: Query) => api.get<ListResult<AdminUser>>("/admin/users", query),
  createUser: (input: { username: string; nickname: string; password: string; role: "admin" | "user" }) =>
    api.post<{ user: AdminUser }>("/admin/users", input).then((r) => r.user),
  approveUser: (id: string) => api.post<void>(`/admin/users/${id}/approve`),
  disableUser: (id: string) => api.post<void>(`/admin/users/${id}/disable`),
  enableUser: (id: string) => api.post<void>(`/admin/users/${id}/enable`),
  setRole: (id: string, role: "admin" | "user") =>
    api.post<void>(`/admin/users/${id}/role`, { role }),
  resetPassword: (id: string) =>
    api.post<{ temporary_password: string }>(`/admin/users/${id}/reset-password`),
  deleteUser: (id: string) => api.delete<void>(`/admin/users/${id}`),

  skills: (query?: Query) => api.get<ListResult<AdminSkillItem>>("/admin/skills", query),
  designs: (query?: Query) => api.get<ListResult<AdminDesignItem>>("/admin/designs", query),
  setSkillOfficial: (id: string, official: boolean) =>
    api.post<void>(`/admin/skills/${id}/${official ? "official" : "unofficial"}`),
  setDesignOfficial: (id: string, official: boolean) =>
    api.post<void>(`/admin/designs/${id}/${official ? "official" : "unofficial"}`),
  approveSkill: (id: string) => api.post<void>(`/admin/skills/${id}/approve`),
  approveDesign: (id: string) => api.post<void>(`/admin/designs/${id}/approve`),

  settings: () => api.get<{ settings: Settings }>("/admin/settings").then((r) => r.settings),
  saveSettings: (patch: Settings) =>
    api.put<{ settings: Settings }>("/admin/settings", { settings: patch }).then((r) => r.settings),
  regenerateSecret: () => api.post<{ ok: boolean }>("/admin/settings/secret/regenerate"),
};
