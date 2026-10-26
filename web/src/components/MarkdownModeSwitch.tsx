export function MarkdownModeSwitch({ mode, onChange }: {
  mode: "preview" | "raw";
  onChange: (mode: "preview" | "raw") => void;
}) {
  return <div className="preview-switch" role="group" aria-label="文档显示方式">
    {(["preview", "raw"] as const).map((value) => (
      <button key={value} type="button" aria-pressed={mode === value}
        className={mode === value ? "active" : ""} onClick={() => onChange(value)}>
        {value === "preview" ? "预览" : "源码"}
      </button>
    ))}
  </div>;
}
