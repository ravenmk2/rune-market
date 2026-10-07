import { api } from "./client";

/** 通用二进制上传(docs/design.md §8.4):raw body,供预览图等多文件场景预上传 */

export interface BlobResult {
  sha256: string;
  size: number;
}

export const blobsApi = {
  upload: (data: Blob) => api.postRaw<BlobResult>("/blobs", data),
};
