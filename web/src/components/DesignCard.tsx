import { Link, useLocation } from "react-router-dom";
import type { DesignListItem } from "../api/designs";
import { OfficialBadge } from "./Badge";
import { DownloadCount } from "./DownloadCount";

export function DesignCard({ design }: { design: DesignListItem }) {
  const location = useLocation();
  return (
    <article className="card">
      <div className="card-thumb">
        {design.preview_thumb_url ? (
          <img src={design.preview_thumb_url} alt={`${design.name} 预览`} loading="lazy" />
        ) : (
          <span className="thumb-placeholder">{design.name.charAt(0).toUpperCase()}</span>
        )}
      </div>
      <div className="card-top">
        <div>
          <div className="card-title">
            <Link className="card-main-link" to={`/d/${design.namespace}/${design.name}`} state={{ marketUrl: location.pathname + location.search }}>{design.name}</Link> {design.official && <OfficialBadge />}
          </div>
          <div className="card-owner">
            <Link className="owner-link" to={`/u/${design.owner.username}`}>
              {design.owner.nickname || design.owner.username}
            </Link>{" "}
            · v{design.latest_version}
          </div>
        </div>
      </div>
      <p className="card-desc">{design.summary}</p>
      <div className="card-foot">
        <span>{design.tags.length > 0 ? design.tags.join(" · ") : "设计系统"}</span>
        <span className="spacer"></span>
        <DownloadCount count={design.download_count} />
      </div>
    </article>
  );
}
