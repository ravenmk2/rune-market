/** 所有技能入口共用的图标；未上传时使用统一 SVG。 */
export function SkillIcon({ url, className = "card-icon", lazy = false }: {
  url?: string | null;
  className?: string;
  lazy?: boolean;
}) {
  return (
    <div className={className}>
      {url ? (
        <img src={url} alt="" loading={lazy ? "lazy" : undefined} />
      ) : (
        <svg width="100%" height="100%" viewBox="0 0 48 48" fill="none" aria-hidden="true">
          <rect width="48" height="48" rx="11" fill="var(--skill-icon-bg)" />
          <g stroke="var(--skill-icon-fg)" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
            <path d="M14 11h15a5 5 0 0 1 5 5v16M14 11a4 4 0 0 0-4 4v4h8v-4a4 4 0 0 0-4-4Zm4 8v14a5 5 0 0 0 5 5" />
            <path d="M23 38h10a5 5 0 0 0 5-5v-1H28v1a5 5 0 0 1-5 5Z" />
            <path d="M22 21h8M22 26h6" />
          </g>
        </svg>
      )}
    </div>
  );
}
