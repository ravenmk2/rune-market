import { Link, useLocation } from "react-router-dom";
import type { SkillListItem } from "../api/skills";
import { OfficialBadge } from "./Badge";
import { Tag } from "./Tag";
import { harnessLabel } from "../utils/format";
import { DownloadCount } from "./DownloadCount";
import { SkillIcon } from "./SkillIcon";

export function SkillCard({ skill }: { skill: SkillListItem }) {
  const location = useLocation();
  const harnessText =
    skill.harnesses && skill.harnesses.length > 0
      ? skill.harnesses.map(harnessLabel).join(" · ")
      : "通用";

  return (
    <article className="card skill-card">
      <div className="card-top">
        <SkillIcon url={skill.icon_url} lazy />
        <div>
          <div className="card-title">
            <Link className="card-main-link" to={`/s/${skill.namespace}/${skill.name}`} state={{ marketUrl: location.pathname + location.search }}>{skill.display_name || skill.name}</Link>{skill.official && <OfficialBadge />}
          </div>
          <div className="skill-original-name">{skill.name}</div>
          <div className="card-owner">
            <Link className="owner-link" to={`/u/${skill.owner.username}`}>
              {skill.owner.nickname || skill.owner.username}
            </Link>{" "}
            · v{skill.latest_version}
          </div>
        </div>
      </div>
      {skill.tags.length > 0 && (
        <div className="stack">
          {skill.tags.map((t) => (
            <Tag key={t} label={t} />
          ))}
        </div>
      )}
      <div className="card-foot">
        <span>{harnessText}</span>
        <span className="spacer"></span>
        <DownloadCount count={skill.download_count} />
      </div>
    </article>
  );
}
