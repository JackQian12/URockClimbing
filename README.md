# URock 攀岩会员小程序

微信小程序 + Go 后端的模块化单体项目。产品范围、业务规则和验收标准见 [PRD.md](./PRD.md)。

## 目录

```text
backend/                 Go API、领域模块、migration、OpenAPI
miniprogram/             微信原生小程序（TypeScript）
admin-web/               React + TypeScript Web 管理后台
deploy/nginx/            本地/服务器反向代理配置
docker-compose.yml       MySQL、API、Nginx、migration
```

## 本地启动

要求：Go 1.24+、Node.js 20+、Docker 和微信开发者工具。

```bash
cp .env.example .env
make setup
docker compose up -d mysql
make migrate
make up
curl http://localhost:8080/health/live
curl http://localhost:8080/health/ready
```

`.env.example` 中的密码仅为字段示例。首次启动前必须在 `.env` 中换成自己的随机密码；生产环境不得复用本地配置。

## 微信小程序

1. 在微信开发者工具中导入 `miniprogram/`。
2. 将 `project.config.json` 中的 `touristappid` 替换为开发 AppID，或使用本地私有项目配置。
3. 本地开发可在开发者工具中临时关闭“校验合法域名”，体验版和生产版必须使用 `https://api.urockclimbing.cn`。
4. 运行 `make typecheck` 做 TypeScript 检查。

当前页面只提供 M0 导航与请求层骨架；后续按 PRD 的 M1 接入真实会员登录。

## Web 管理后台

后台构建后由 Nginx 托管在 `/admin/`，管理员登录、会话校验、退出及首次登录强制改密已接入 Go API。会员、次卡、订单退款、核销、员工和审计页面当前为可扩展的功能骨架，业务 CRUD 接口将在对应里程碑实现。

本地开发：

```bash
cd admin-web
npm run dev
```

首次部署后，通过服务器内的 CLI 创建管理员。密码至少 12 个字符，且首次登录必须修改：

```bash
docker compose exec \
  -e ADMIN_USERNAME=your_admin \
  -e ADMIN_PASSWORD='your_initial_password' \
  api /app/urock-admin create
```

生产后台只允许在 HTTPS 配置完成后登录，避免账号密码通过明文 HTTP 传输。

## 数据库 migration

```bash
make migrate
make migrate-down
```

生产环境执行 migration 前必须备份数据库。应用启动不会自动修改数据库结构。

## 质量检查

```bash
make check
```

检查包含 Go 格式、race test、`go vet`、小程序类型检查和 Web 后台生产构建。

## 环境与密钥

- 只提交 `.env.example`，不提交 `.env`。
- 微信 AppSecret、商户私钥、APIv3 密钥只配置在服务端。
- 生产服务器仅开放 80/443；MySQL 不开放公网端口。
- 当前未使用 COS，数据库备份必须复制到服务器之外的独立存储位置。

## 生产服务器

生产服务器 SSH 信息保存在 `deploy/ssh_config`，只包含地址、用户名和本机密钥路径，不保存密码。

```bash
ssh -F deploy/ssh_config urock-production
```

当前远程部署目录为 `/opt/urock`，生产配置保存在服务器 `/opt/urock/.env`，权限为 `600`。更新部署前应先备份数据库，再上传代码、执行 migration 并重建容器。
