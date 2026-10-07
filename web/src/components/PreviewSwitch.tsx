export type PreviewMode = "desktop" | "mobile";

export function PreviewSwitch({
  mode,
  onChange,
}: {
  mode: PreviewMode;
  onChange: (mode: PreviewMode) => void;
}) {
  return (
    <div className="preview-switch">
      <span className={mode === "desktop" ? "active" : ""} onClick={() => onChange("desktop")}>
        Desktop
      </span>
      <span className={mode === "mobile" ? "active" : ""} onClick={() => onChange("mobile")}>
        Mobile
      </span>
    </div>
  );
}
