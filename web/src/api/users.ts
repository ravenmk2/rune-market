import { api } from "./client";
import type { SkillListItem } from "./skills";
import type { DesignListItem } from "./designs";

/** 用户公开主页(docs/design.md §8.2) */

export interface PublicUser {
  username: string;
  nickname: string;
  bio: string;
  has_avatar: boolean;
  created_at: string;
  /** 契约未含;后端若返回则用于头像 URL(/avatars/{id}.png) */
  id?: string;
}

export interface UserProfile {
  user: PublicUser;
  skills: SkillListItem[];
  designs: DesignListItem[];
}

export const usersApi = {
  profile: (username: string) => api.get<UserProfile>(`/users/${username}`),
};
