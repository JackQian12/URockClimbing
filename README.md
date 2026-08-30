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
2. `project.config.json` 已配置 URock 正式 AppID。
3. 本地开发可在开发者工具中临时关闭“校验合法域名”，体验版和生产版必须使用 `https://api.urockclimbing.cn`。
4. 运行 `make typecheck` 做 TypeScript 检查。

为保证真机预览可用，`develop`、`trial` 和 `release` 默认都请求线上 HTTPS API。如需在微信开发者工具联调本地 API，可在调试器控制台执行：

```js
wx.setStorageSync('urock.development_api_base_url', 'http://127.0.0.1:8080/api/v1')
```

联调完成后执行 `wx.removeStorageSync('urock.development_api_base_url')` 恢复线上 API。

代码上传私钥仅保存在本机 `miniprogram/.secrets/`，该目录已被 Git 忽略。CI 仅用于生成预览码，禁止使用 `miniprogram-ci` 上传，否则微信后台会把开发者标记为“CI机器人1”。正式上传必须通过已登录 **Jack** 开发者账号的微信开发者工具：

```bash
cd miniprogram
npm run preview -- --desc="URock preview"
npm run upload -- --version=0.1.0 --desc="Jack: Initial member mini program"
```

`npm run upload` 会调用本机微信开发者工具 CLI，并在未登录时拒绝上传。每次上传前必须确认开发者工具当前登录账号为 Jack；不得恢复 CI 上传入口。

小程序已接入真实微信登录、令牌刷新、手机号授权注册和会员昵称维护。微信登录后只建立待注册账号，用户授权手机号成功才成为正式会员。本地联调可在非生产环境显式设置 `WECHAT_LOGIN_MOCK=true`，生产环境禁止启用。

手机号能力使用前，需在微信公众平台确认小程序主体已认证、手机号快速验证额度可用，并在「用户隐私保护指引」中声明手机号用于会员注册、身份核对和到店联系。完整号码由 Go API 加密入库，不得写入日志。

## Web 管理后台

后台构建后由 Nginx 托管在 `/admin/`，管理员登录、会话校验、退出及首次登录强制改密已接入 Go API。会员卡商品支持次卡以及 7/30/90/365 天期限卡；会员、员工和审计管理已接入，订单支付、真实退款和核销闭环仍按 PRD 后续里程碑实现。

微信支付商户审核期间，商品详情和会员订单能力可以正常开发与验收，但小程序支付入口保持关闭，后端支付准备接口也不会创建支付流水。审核通过后的 Secret、实现顺序与上线门禁见 [微信支付准备清单](docs/wechat-pay-readiness.md)。

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
