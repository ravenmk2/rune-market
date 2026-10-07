import { Link } from "react-router-dom";
import type { DesignListItem } from "../api/designs";
import { OfficialBadge } from "./Badge";
import { formatCount } from "../utils/format";

const TINTS = ["tint-a", "tint-b", "tint-c", "tint-d"];

export function DesignCard({ design, index = 0 }: { design: DesignListItem; index?: number }) {
  return (
    <Link className="card" to={`/d/${design.namespace}/${design.name}`}>
      <div className="card-thumb">
        {design.preview_thumb_url ? (
          <img src={design.preview_thumb_url} alt={`${design.name} 预览`} loading="lazy" />
        ) : (
          <span className="thumb-placeholder">{design.name.charAt(0).toUpperCase()}</span>
        )}
      </div>
      <div className="card-top">
        <div className={`card-icon ${TINTS[index % TINTS.length]}`}>
          {design.name.charAt(0).toUpperCase()}
        </div>
        <div>
          <div className="card-title">
            {design.name} {design.official && <OfficialBadge />}
          </div>
          <div className="card-owner">
            {design.owner.nickname || design.owner.username} · v{design.latest_version}
          </div>
        </div>
      </div>
      <p className="card-desc">{design.summary}</p>
      <div className="card-foot">
        <span>{design.tags.length > 0 ? design.tags.join(" · ") : "设计系统"}</span>
        <span className="spacer"></span>
        <span>↓ {formatCount(design.download_count)}</span>
      </div>
    </Link>
  );
}
