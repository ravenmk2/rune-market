import { api } from "./client";

/** 安装向导(docs/design.md §8.1,仅 setup 模式可用;启动探测走 /site,见 App.tsx) */

export type AppMode = "setup" | "normal";

export type DatabaseDriver = "sqlite" | "mysql";

export interface SqliteDatabaseConfig {
  driver: "sqlite";
  data_dir: string;
}

export interface MysqlDatabaseConfig {
  driver: "mysql";
  host: string;
  database: string;
  username: string;
  password: string;
}

export type DatabaseConfig = SqliteDatabaseConfig | MysqlDatabaseConfig;

export interface AdminInput {
  username: string;
  nickname: string;
  password: string;
}

export interface TestConnectionResult {
  ok: boolean;
}

export interface AdminResult {
  ok: boolean;
  /** 安装完成后服务端切换到正常模式 */
  mode: AppMode;
}

export const setupApi = {
  testDatabase: (config: MysqlDatabaseConfig) =>
    api.post<TestConnectionResult>("/setup/database/test", config),
  saveDatabase: (config: DatabaseConfig) => api.post<void>("/setup/database", config),
  createAdmin: (input: AdminInput) => api.post<AdminResult>("/setup/admin", input),
};
