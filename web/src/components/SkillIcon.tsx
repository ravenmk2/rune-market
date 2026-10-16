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
          <rect width="48" height="48" rx="11" fill="#F6E9E2" />
          <g stroke="#C15F3C" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
            <path d="M24 12H16a3 3 0 0 0-3 3v18a3 3 0 0 0 3 3h16a3 3 0 0 0 3-3V23" />
            <path d="M19 23h7M19 29h10" />
            <path d="m33 9 1.4 3.6L38 14l-3.6 1.4L33 19l-1.4-3.6L28 14l3.6-1.4L33 9Z" fill="#F6E9E2" />
          </g>
        </svg>
      )}
    </div>
  );
}
