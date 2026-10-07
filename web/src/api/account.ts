import { api } from "./client";
import type { User } from "./auth";

/** 账号管理(docs/design.md §8.2) */

export interface UpdateAccountInput {
  nickname: string;
  bio: string;
}

export const accountApi = {
  /** 改昵称、bio */
  update: (input: UpdateAccountInput) =>
    api.put<{ user: User }>("/account", input).then((r) => r.user),
  /** 改密码:校验旧密码,成功后后端作废旧会话 */
  changePassword: (oldPassword: string, newPassword: string) =>
    api.put<void>("/account/password", { old_password: oldPassword, new_password: newPassword }),
  /** 上传头像:raw body = 图片(PNG/JPG ≤5MB),归一化 PNG + 32/64/128 缩略图 */
  uploadAvatar: (image: Blob) =>
    api.postRaw<{ user: User }>("/account/avatar", image).then((r) => r.user),
  /** 移除头像,回落字母占位 */
  removeAvatar: () => api.delete<{ user: User }>("/account/avatar").then((r) => r.user),
  /** 注销账号(创始用户 403 founder_protected) */
  deleteAccount: () => api.delete<{ ok?: boolean }>("/account"),
};
