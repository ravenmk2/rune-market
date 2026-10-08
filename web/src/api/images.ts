import { api } from "./client";

/** 通用图片上传(docs/design.md §8.4):raw body PNG/JPEG,服务端保留原始格式 */

export interface ImageResult {
  sha256: string;
  ext: string;
  size: number;
  url: string;
  thumb_url: string;
}

export const imagesApi = {
  upload: (data: Blob) => api.postRaw<ImageResult>("/images", data),
};
