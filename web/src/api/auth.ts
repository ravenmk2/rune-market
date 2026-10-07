import { api } from "./client";

/** 认证与账号(docs/design.md §8.2) */

export type UserRole = "admin" | "user";
export type UserStatus = "active" | "pending" | "disabled";

export interface User {
  id: string;
  username: string;
  nickname: string;
  role: UserRole;
  status: UserStatus;
  has_avatar: boolean;
  is_founder?: boolean;
  updated_at?: string;
}

export interface RegisterInput {
  username: string;
  nickname: string;
  password: string;
}

/** 认证接口响应统一包裹在 {"user": ...} 中 */
interface UserEnvelope {
  user: User;
}

export const authApi = {
  me: () => api.get<UserEnvelope>("/auth/me").then((r) => r.user),
  login: (username: string, password: string) =>
    api.post<UserEnvelope>("/auth/login", { username, password }).then((r) => r.user),
  logout: () => api.post<void>("/auth/logout"),
  /** registration_mode=approval 时返回 user.status 为 "pending",需管理员批准后方可登录 */
  register: (input: RegisterInput) =>
    api.post<UserEnvelope>("/auth/register", input).then((r) => r.user),
};
