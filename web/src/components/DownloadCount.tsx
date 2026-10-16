import { formatCount } from "../utils/format";

export function DownloadCount({ count }: { count: number }) {
  const label = `下载次数：${count.toLocaleString("zh-CN")}`;
  return (
    <span className="download-count" title={label} aria-label={label}>
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
        <path d="M12 3v12m-4-4 4 4 4-4M4 15v4a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-4" />
      </svg>
      <span>{formatCount(count)}</span>
    </span>
  );
}
