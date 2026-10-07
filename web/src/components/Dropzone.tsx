import { useRef, useState } from "react";
import type { ChangeEvent, DragEvent } from "react";

interface DropzoneProps {
  accept?: string;
  title: string;
  sub?: string;
  disabled?: boolean;
  onFile: (file: File) => void;
}

/** 拖拽 + 点击选文件(raw body 上传,不用 FormData,§8) */
export function Dropzone({ accept, title, sub, disabled, onFile }: DropzoneProps) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [over, setOver] = useState(false);

  function pick(e: ChangeEvent<HTMLInputElement>) {
    const f = e.target.files?.[0];
    if (f) onFile(f);
    e.target.value = "";
  }

  function drop(e: DragEvent) {
    e.preventDefault();
    setOver(false);
    if (disabled) return;
    const f = e.dataTransfer.files?.[0];
    if (f) onFile(f);
  }

  return (
    <div
      className="dropzone"
      style={over ? { borderColor: "var(--accent)", background: "#FDFBF8" } : undefined}
      onClick={() => !disabled && inputRef.current?.click()}
      onDragOver={(e) => {
        e.preventDefault();
        if (!disabled) setOver(true);
      }}
      onDragLeave={() => setOver(false)}
      onDrop={drop}
    >
      <div className="dz-title">{title}</div>
      {sub && <div className="dz-sub">{sub}</div>}
      <input
        ref={inputRef}
        type="file"
        accept={accept}
        style={{ display: "none" }}
        onChange={pick}
      />
    </div>
  );
}
