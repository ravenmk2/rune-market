import { useCallback, useState } from "react";
import Cropper from "react-easy-crop";
import type { Area } from "react-easy-crop";

interface AvatarCropDialogProps {
  imageSrc: string;
  busy: boolean;
  onCancel: () => void;
  onConfirm: (blob: Blob) => void;
}

function loadImage(src: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const img = new Image();
    img.onload = () => resolve(img);
    img.onerror = () => reject(new Error("图片加载失败"));
    img.src = src;
  });
}

/** 按裁剪区域从原图导出方形 PNG,边长不超过 512px */
async function cropToBlob(src: string, area: Area): Promise<Blob> {
  const img = await loadImage(src);
  const size = Math.min(512, Math.round(area.width), Math.round(area.height));
  const canvas = document.createElement("canvas");
  canvas.width = size;
  canvas.height = size;
  const ctx = canvas.getContext("2d");
  if (!ctx) throw new Error("当前浏览器不支持 canvas 导出");
  ctx.drawImage(img, area.x, area.y, area.width, area.height, 0, 0, size, size);
  return new Promise((resolve, reject) =>
    canvas.toBlob(
      (b) => (b ? resolve(b) : reject(new Error("头像导出失败"))),
      "image/png",
    ),
  );
}

/** 头像前端裁剪弹窗:拖动 + 缩放,确认后导出方形 PNG 再上传 */
export function AvatarCropDialog({ imageSrc, busy, onCancel, onConfirm }: AvatarCropDialogProps) {
  const [crop, setCrop] = useState({ x: 0, y: 0 });
  const [zoom, setZoom] = useState(1);
  const [area, setArea] = useState<Area | null>(null);
  const [error, setError] = useState("");

  const onCropComplete = useCallback((_: Area, pixels: Area) => setArea(pixels), []);

  async function confirm() {
    if (!area || busy) return;
    setError("");
    try {
      onConfirm(await cropToBlob(imageSrc, area));
    } catch (e) {
      setError(e instanceof Error ? e.message : "头像处理失败,请重试");
    }
  }

  return (
    <div className="dialog-mask" onClick={busy ? undefined : onCancel}>
      <div className="dialog" onClick={(e) => e.stopPropagation()}>
        <h2>裁剪头像</h2>
        <div className="crop-area">
          <Cropper
            image={imageSrc}
            crop={crop}
            zoom={zoom}
            aspect={1}
            cropShape="round"
            showGrid={false}
            onCropChange={setCrop}
            onZoomChange={setZoom}
            onCropComplete={onCropComplete}
          />
        </div>
        <div className="crop-zoom">
          <span>缩放</span>
          <input
            type="range"
            min={1}
            max={3}
            step={0.05}
            value={zoom}
            aria-label="缩放"
            onChange={(e) => setZoom(Number(e.target.value))}
          />
        </div>
        {error && (
          <div className="alert err" style={{ marginBottom: 14 }}>
            <span>!</span>
            <span>{error}</span>
          </div>
        )}
        <div className="dialog-actions">
          <button className="btn btn-outline" type="button" disabled={busy} onClick={onCancel}>
            取消
          </button>
          <button className="btn btn-primary" type="button" disabled={busy} onClick={confirm}>
            {busy ? "上传中…" : "确认上传"}
          </button>
        </div>
      </div>
    </div>
  );
}
