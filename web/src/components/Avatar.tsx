import type { User } from "../api/auth";

interface AvatarProps {
  user: User;
  /** 缩略图尺寸:32 / 64 / 128(docs/design.md §8.2) */
  size?: 32 | 64 | 128;
}

/** 头像:has_avatar 为真输出 /avatars/{id}_{size}.png?v={updated_at},否则字母占位(§12) */
export function Avatar({ user, size = 32 }: AvatarProps) {
  const initial = (user.nickname || user.username || "?").trim().charAt(0).toUpperCase();
  return (
    <span className="avatar" style={{ width: size, height: size }}>
      {user.has_avatar ? (
        <img
          src={`/avatars/${user.id}_${size}.png?v=${encodeURIComponent(user.updated_at ?? "")}`}
          alt={user.nickname || user.username}
        />
      ) : (
        initial
      )}
    </span>
  );
}
