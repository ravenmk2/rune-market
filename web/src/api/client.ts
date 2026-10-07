/**
 * 统一 fetch 封装(契约见 docs/design.md §8):
 * - 统一前缀 /api/v1
 * - 错误格式 {"error": {"code", "message", "details"}}
 * - 修改类请求携带 X-Requested-With: XMLHttpRequest(CSRF 校验,§7)
 * - 上传不用 multipart,raw body 即文件数据,元数据走 query string
 */

export const API_BASE = "/api/v1";

export interface ApiErrorBody {
  code: string;
  message: string;
  details?: unknown[];
}

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly details?: unknown[];

  constructor(status: number, body: ApiErrorBody) {
    super(body.message);
    this.name = "ApiError";
    this.status = status;
    this.code = body.code;
    this.details = body.details;
  }
}

export type Query = Record<string, string | number | boolean | undefined | null>;

/** 401 回调:由 AuthContext 注册,用于清空会话状态并跳登录页 */
let unauthorizedHandler: (() => void) | null = null;

export function setUnauthorizedHandler(handler: (() => void) | null): void {
  unauthorizedHandler = handler;
}

function buildQuery(query?: Query): string {
  if (!query) return "";
  const params = new URLSearchParams();
  for (const [k, v] of Object.entries(query)) {
    if (v !== undefined && v !== null && v !== "") params.set(k, String(v));
  }
  const s = params.toString();
  return s ? `?${s}` : "";
}

interface RequestOptions {
  json?: unknown;
  raw?: BodyInit;
  contentType?: string;
  query?: Query;
}

async function request<T>(method: string, path: string, opts: RequestOptions = {}): Promise<T> {
  const headers: Record<string, string> = { "X-Requested-With": "XMLHttpRequest" };
  let body: BodyInit | undefined;

  if (opts.raw !== undefined) {
    body = opts.raw;
    if (opts.contentType) headers["Content-Type"] = opts.contentType;
  } else if (opts.json !== undefined) {
    headers["Content-Type"] = "application/json";
    body = JSON.stringify(opts.json);
  }

  const res = await fetch(`${API_BASE}${path}${buildQuery(opts.query)}`, {
    method,
    headers,
    body,
    credentials: "same-origin",
  });

  // /auth/* 用于探测/建立会话,401 属正常业务结果,不触发全局跳转
  if (res.status === 401 && !path.startsWith("/auth/")) {
    unauthorizedHandler?.();
  }

  if (!res.ok) {
    let errBody: ApiErrorBody = { code: "unknown", message: `请求失败(${res.status})` };
    try {
      const data = (await res.json()) as { error?: ApiErrorBody };
      if (data && data.error && typeof data.error.message === "string") errBody = data.error;
    } catch {
      // 非 JSON 错误体,保留默认
    }
    throw new ApiError(res.status, errBody);
  }

  if (res.status === 204) return undefined as T;
  const contentType = res.headers.get("content-type") ?? "";
  if (contentType.includes("application/json")) return (await res.json()) as T;
  return (await res.text()) as T;
}

export const api = {
  get: <T>(path: string, query?: Query) => request<T>("GET", path, { query }),
  post: <T>(path: string, json?: unknown, query?: Query) => request<T>("POST", path, { json, query }),
  put: <T>(path: string, json?: unknown, query?: Query) => request<T>("PUT", path, { json, query }),
  delete: <T>(path: string, query?: Query) => request<T>("DELETE", path, { query }),
  /** raw body 上传(§8 上传约定):body 即文件数据,元数据走 query */
  postRaw: <T>(path: string, raw: BodyInit, query?: Query, contentType?: string) =>
    request<T>("POST", path, { raw, query, contentType }),
};
