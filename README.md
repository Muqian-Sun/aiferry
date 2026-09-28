# AiFerry

AiFerry 是一个 AI API 网关平台：统一接入上游资源（OAuth 成品号，以及官方 / 聚合平台的 API Key），向用户发放平台自己的 API Key，负责鉴权、按 Token 计费、调度与转发。对外提供 OpenAI（`/v1/chat/completions`、`/v1/responses`）、Anthropic（`/v1/messages`）、Gemini 等兼容接口。

系统分两个入口：用户站（默认端口 8080）和管理站（默认端口 8081，compose 默认只绑定本机）。

技术栈：Go（Gin + Ent）后端、Vue 3 前端，PostgreSQL + Redis。

## 部署（Docker Compose）

只支持 Docker Compose 部署。镜像 `aiferry:${AIFERRY_VERSION:-latest}` 由本仓库根目录的 `Dockerfile` 在本地构建，不从任何镜像仓库拉取。

```bash
git clone https://github.com/Muqian-Sun/tokenferry.git aiferry
cd aiferry/deploy

cp .env.example .env
chmod 600 .env
# 至少设置 POSTGRES_PASSWORD；JWT_SECRET、TOTP_ENCRYPTION_KEY 建议用 openssl rand -hex 32 生成固定值，
# 留空会在每次启动时随机生成（登录态与两步验证会失效）

docker compose up -d --build
docker compose logs -f aiferry
```

首次启动自动执行数据库迁移并创建管理员：邮箱取 `ADMIN_EMAIL`（默认 `admin@aiferry.local`），`ADMIN_PASSWORD` 留空时随机生成并打印在日志里。

`deploy/` 下的 compose 文件：

| 文件 | 用途 |
|------|------|
| `docker-compose.yml` | 应用 + PostgreSQL + Redis，数据放命名卷 |
| `docker-compose.local.yml` | 同上，数据放 `deploy/` 下的本地目录，便于整体迁移 |
| `docker-compose.standalone.yml` | 只跑应用，PostgreSQL / Redis 由外部提供 |
| `docker-compose.dev.yml` | 本地开发构建 |

环境变量、反向代理、迁移与排错见 [deploy/README.md](deploy/README.md)，边缘安全见 [deploy/EDGE_SECURITY.md](deploy/EDGE_SECURITY.md)。

### 升级

```bash
git pull
cd deploy
docker compose up -d --build
```

数据库迁移在应用启动时自动执行。也可以用 `deploy/build_image.sh` 先构建出 `aiferry:<版本>`，再在 `.env` 里设 `AIFERRY_VERSION=<版本>` 固定版本。版本号取 `backend/cmd/server/VERSION`（在打了精确 tag 的提交上构建时取 tag）。

## 开发

开发环境、CI、常见坑与提交前检查见 [DEV_GUIDE.md](DEV_GUIDE.md)。

## 许可证

本项目基于 [sub2api](https://github.com/Wei-Shaw/sub2api)（LGPL-3.0，Copyright (c) 2026 Wesley Liddick）修改，按 GNU Lesser General Public License v3.0 发布；[LICENSE](LICENSE) 保留原文。
