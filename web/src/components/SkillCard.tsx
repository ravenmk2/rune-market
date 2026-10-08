import { Link, useNavigate } from "react-router-dom";
import type { SkillListItem } from "../api/skills";
import { OfficialBadge } from "./Badge";
import { Tag } from "./Tag";
import { formatCount, harnessLabel } from "../utils/format";

const TINTS = ["tint-a", "tint-b", "tint-c", "tint-d"];

export function SkillCard({ skill, index = 0 }: { skill: SkillListItem; index?: number }) {
  const navigate = useNavigate();
  const harnessText =
    skill.harnesses && skill.harnesses.length > 0
      ? skill.harnesses.map(harnessLabel).join(" · ")
      : "通用";

  return (
    <Link className="card" to={`/s/${skill.namespace}/${skill.name}`}>
      <div className="card-top">
        {skill.icon_url ? (
          <div className="card-icon">
            <img src={skill.icon_url} alt="" loading="lazy" />
          </div>
        ) : (
          <div className={`card-icon ${TINTS[index % TINTS.length]}`}>
            {skill.name.charAt(0).toUpperCase()}
          </div>
        )}
        <div>
          <div className="card-title">
            {skill.name} {skill.official && <OfficialBadge />}
          </div>
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
      <p className="card-desc">{skill.summary}</p>
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
        <span>↓ {formatCount(skill.download_count)}</span>
      </div>
    </Link>
  );
}
