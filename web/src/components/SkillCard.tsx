import { Link, useNavigate } from "react-router-dom";
import type { SkillListItem } from "../api/skills";
import { OfficialBadge } from "./Badge";
import { Tag } from "./Tag";
import { harnessLabel } from "../utils/format";
import { DownloadCount } from "./DownloadCount";
import { SkillIcon } from "./SkillIcon";

export function SkillCard({ skill }: { skill: SkillListItem }) {
  const navigate = useNavigate();
  const harnessText =
    skill.harnesses && skill.harnesses.length > 0
      ? skill.harnesses.map(harnessLabel).join(" · ")
      : "通用";

  return (
    <Link className="card skill-card" to={`/s/${skill.namespace}/${skill.name}`}>
      <div className="card-top">
        <SkillIcon url={skill.icon_url} lazy />
        <div>
          <div className="card-title">
            {skill.display_name || skill.name} {skill.official && <OfficialBadge />}
          </div>
          <div className="skill-original-name">{skill.name}</div>
          <div className="card-owner">
            <span
              className="owner-link"
              onClick={(e) => {
                e.preventDefault();
                e.stopPropagation();
                navigate(`/u/${skill.owner.username}`);
              }}
            >
              {skill.owner.nickname || skill.owner.username}
            </span>{" "}
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
    </Link>
  );
}
