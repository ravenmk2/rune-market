export type PreviewMode = "desktop" | "mobile";

export function PreviewSwitch({
  mode,
  onChange,
  available,
}: {
  mode: PreviewMode;
  onChange: (mode: PreviewMode) => void;
  available?: PreviewMode[];
}) {
  return (
    <div className="preview-switch" role="group" aria-label="预览设备">
      {(["desktop", "mobile"] as const).map((device) => {
        const disabled = !!available && !available.includes(device);
        const label = device === "desktop" ? "桌面" : "移动";
        return <button key={device} type="button" aria-pressed={mode === device}
          className={mode === device ? "active" : ""} disabled={disabled}
          title={disabled ? `未上传${label}端预览` : undefined}
          onClick={() => onChange(device)}>{label}</button>;
      })}
    </div>
  );
}
