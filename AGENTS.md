# AI Agents 工作规范

RuneMarket:Agent Skills 与 DESIGN.md 的制品商店(类 Docker Hub),Go(Gin + Logrus + SQLite/MySQL)单体后端 embed React 前端。

## 目录结构

```txt
rune-market/
├── cmd/runemarket/    # 程序入口
├── internal/          # 应用私有代码(config/secret/store/server/auth/skillpkg/designmd/blob/hub)
├── web/               # React 前端源码与 embed 产物(dist/ 不入库)
├── docs/              # 设计文档
├── scripts/           # 辅助脚本
└── data/              # 运行时数据(config.toml / secret / blobs / images / *.db),不入库
```

## 文档索引

文档变化时同步更新

- docs/design.md:详细设计(数据模型、API、校验管线、工程化、里程碑),开发依据
- docs/prototypes/market-theme.html:纸白暖灰风格静态样式提案,含明暗主题与技能图标配色对比,非正式页面
