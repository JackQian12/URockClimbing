# URock 攀岩会员小程序 PRD

> 文档用途：本 PRD 主要供 AI Agent 进行产品设计、技术设计、任务拆分、编码和验收。除非文档明确标记为“待确认”，实现时不得自行扩展业务范围或改变核心规则。

## 0. 文档信息

| 项目 | 内容 |
|---|---|
| 产品名称 | URock 攀岩会员小程序 |
| 产品形态 | 微信小程序 + Web 管理后台 + Go 后端 API |
| 当前阶段 | MVP / V1 |
| 核心目标 | 跑通会员注册、次卡购买、次卡查看、到店核销闭环 |
| 目标用户 | 攀岩馆会员、前台/核销员工、管理员 |
| 部署形态 | 单台中国内地 Linux 服务器，Docker Compose 部署 |
| 域名规划 | `api.urockclimbing.cn`（API）、`admin.urockclimbing.cn`（管理后台）、`urockclimbing.cn`（协议及品牌页，可后续实现） |
| 数据库 | MySQL 8.0 |
| 文件存储 | V1 不接入 COS，不实现用户文件上传；静态图片随小程序发布 |
| 文档版本 | 1.0 |
| 更新日期 | 2026-08-02 |

## 1. 产品背景

URock 需要一套面向攀岩馆会员的微信小程序。当前会员数量和访问量较小，首期优先降低开发与运维复杂度，但领域模型和代码边界必须支持未来增加：

- 教练与课程预约；
- 攀岩周边商品购买；
- 储值、优惠券、活动；
- 多门店；
- 更复杂的多门店运营后台。

V1 只实现会员、次卡、购买、核销和记录查询，不提前实现未来功能，但数据库主键、订单模型、权限模型和模块划分不得阻碍扩展。

## 2. 产品目标与成功标准

### 2.1 业务目标

1. 用户无需线下建档，可通过微信完成注册。
2. 用户可以查看并购买在售次卡。
3. 微信支付成功后，次卡自动到账且不可重复发放。
4. 用户到店后可以展示一次性核销码。
5. 员工可扫码核销，系统准确扣减一次并留下审计记录。
6. 管理员可管理次卡商品，查看会员、订单、核销数据，并在后台对符合条件的订单发起退款。

### 2.2 MVP 成功标准

- 新用户在 1 分钟内完成登录并进入首页。
- 支付回调成功后 10 秒内在“我的次卡”看到新卡。
- 正常网络下，核销操作 3 秒内返回明确结果。
- 同一核销码并发提交时最多成功一次。
- 每次余额变化均有不可覆盖的业务流水。
- 无管理员权限的用户不能访问商品管理或核销接口。

### 2.3 非目标

以下内容不属于 V1，AI Agent 不得顺带实现：

- 教练预约、排课和课程管理；
- 商品购物车、物流和库存；
- 优惠券、积分、储值余额；
- 用户自助退款、部分退款，以及已发生核销的次卡退款；
- 转赠、共享卡、家庭卡；
- 次卡冻结、延期和人工增减次数；
- 多门店结算；
- 短信验证码和短信通知；
- 用户图片、视频、证件上传；
- 员工使用的独立 Web 工作台；V1 员工扫码核销仍放在小程序，Web 管理后台仅供 ADMIN 使用。

## 3. 角色与权限

### 3.1 角色定义

| 角色 | 标识 | 权限 |
|---|---|---|
| 会员 | `MEMBER` | 查看商品、购卡、查看自己的卡、生成核销码、查看自己的订单和核销记录 |
| 员工 | `STAFF` | 会员全部权限；扫码核销；查询核销结果和当日核销记录 |
| 管理员 | `ADMIN` | 员工全部权限；管理次卡商品；查询全部会员、订单、卡和核销记录；对符合条件的订单发起全额退款；设置员工角色 |

### 3.2 权限原则

- 新注册用户默认角色为 `MEMBER`。
- `STAFF`、`ADMIN` 必须由数据库初始化或管理员授权，禁止用户自行申请或修改。
- 后端必须校验权限；前端隐藏入口不能替代后端鉴权。
- 管理员修改角色必须记录操作日志。
- V1 一个用户只拥有一个最高角色；权限关系为 `ADMIN > STAFF > MEMBER`。

## 4. 关键业务约定

为避免实现歧义，V1 采用以下确定规则：

1. **会员定义**：完成微信登录并创建会员档案即成为会员，不另外收取会员费。
2. **次卡定义**：包含固定可用次数、售价和有效期的商品。一次核销固定扣减 1 次。
3. **生效时间**：支付成功并发卡时立即生效。
4. **过期时间**：`activated_at + validity_days`，精确到秒；到期时刻之后不可核销。
5. **使用顺序**：用户主动选择某一张卡生成核销码，不自动从多张卡中扣除。
6. **退款**：会员端不提供退款按钮或退款 API。退款只能由 `ADMIN` 在管理后台发起。V1 仅支持整单全额退款，且该订单发放的次卡必须从未核销；部分退款和已核销订单退款不支持自动处理。
7. **删除**：商品、订单、会员卡、核销记录均不得物理删除。商品使用上下架状态。
8. **金额**：所有金额以人民币分为整数存储和计算，禁止使用浮点数。
9. **时间**：数据库存 UTC；API 使用 RFC3339；前端以 Asia/Shanghai 展示。
10. **手机号**：V1 为可选资料，不作为登录凭证。若接入微信手机号能力，必须经用户明确授权。
11. **昵称和头像**：不得依赖微信自动提供；用户可不填写，使用默认昵称和本地默认头像。
12. **订单快照**：商品改名、改价、下架不能影响历史订单和已发放次卡。

## 5. 核心用户流程

### 5.1 首次登录

1. 用户打开小程序。
2. 小程序调用 `wx.login` 获取临时 `code`。
3. 前端将 `code` 发送至后端。
4. 后端调用微信 `code2Session` 获取 `openid` 和 `session_key`，不得将 `session_key` 返回前端。
5. 后端按 `openid + appid` 查找身份；不存在则创建用户和会员档案。
6. 后端签发访问令牌和刷新令牌。
7. 前端进入首页。

### 5.2 购买次卡

1. 用户进入“次卡商城”，查看在售商品。
2. 用户选择商品并确认购买。
3. 后端重新读取商品状态和价格，创建 `PENDING` 订单。
4. 后端调用微信支付商户 API 创建预支付单。
5. 后端向前端返回调起支付所需参数。
6. 前端调用 `wx.requestPayment`。
7. 是否支付成功以后端验证过签名的微信支付回调为准，不能以前端结果为准。
8. 后端在同一业务事务中将订单置为 `PAID` 并创建会员卡。
9. 用户进入订单详情或“我的次卡”查看结果。

### 5.3 用户生成核销码

1. 用户进入“我的次卡”，选择一张 `ACTIVE` 且有剩余次数的卡。
2. 用户点击“出示核销码”。
3. 后端校验卡片所有权、状态、有效期和剩余次数。
4. 后端创建随机、一次性、短时有效的核销令牌。
5. 前端展示二维码和剩余有效时间。
6. 令牌默认 5 分钟过期；同一卡再次生成时，旧的未使用令牌立即失效。
7. 二维码只包含不可猜测的随机令牌或短链接，不得包含用户 ID、卡 ID、余额等敏感明文。

### 5.4 员工核销

1. 员工进入“员工工作台”，点击扫码核销。
2. 使用 `wx.scanCode` 读取核销令牌。
3. 前端向后端请求核销预览。
4. 后端返回会员脱敏信息、卡名称、到期时间、当前剩余次数。
5. 员工点击确认核销。
6. 后端在数据库事务中锁定会员卡和核销令牌，再次校验全部条件。
7. 扣减 1 次、写入核销流水、将令牌标记为已使用。
8. 返回核销成功页面，展示扣减前后次数和流水号。
9. 重复提交同一请求应返回第一次的核销结果，不得再次扣减。

### 5.5 管理员退款

1. 管理员在订单管理中查看已支付订单。
2. 管理员点击“申请退款”，系统展示订单金额、次卡状态和退款限制。
3. 管理员填写退款原因并二次确认。
4. 后端校验管理员权限、订单为 `PAID`、会员卡未发生任何核销且不存在进行中的退款。
5. 后端先将订单置为 `REFUNDING`，同时将会员卡置为 `REFUND_LOCKED`，立即禁止生成核销码和核销。
6. 后端调用微信支付退款 API，退款金额固定等于订单实付金额。
7. 微信退款成功通知验签通过后，将订单置为 `REFUNDED`，会员卡置为 `REFUNDED`，记录退款流水和管理员审计日志。
8. 如果退款明确失败，将订单恢复为 `PAID`、会员卡恢复为 `ACTIVE`，并保留失败流水；未知状态必须主动查询微信退款结果，禁止直接恢复卡片。
9. 会员可在订单详情中查看“退款中/已退款”，但不能自行发起或取消退款。

### 5.6 管理员登录 Web 后台

1. 管理员访问 `https://admin.urockclimbing.cn`。
2. 使用管理员用户名和密码登录；账号必须关联一个状态正常且角色为 `ADMIN` 的用户。
3. 后端校验密码、账号锁定状态和用户角色，成功后签发仅用于管理后台的短期会话。
4. 会话通过 `HttpOnly`、`Secure`、`SameSite=Strict` Cookie 保存，前端不得把管理令牌写入 localStorage。
5. 修改数据的请求必须同时校验 CSRF token、管理员角色和资源版本。
6. 连续 5 次密码失败后锁定账号 15 分钟；登录成功、失败、退出和改密均写入安全审计日志。
7. 初始管理员账号通过服务器 CLI 创建，首次登录必须修改随机初始密码；不得在代码或 migration 中硬编码管理员密码。

## 6. 页面与交互需求

### 6.1 TabBar

V1 使用三个 Tab：

1. **首页**：会员概要、可用次卡、快捷购卡、最近核销。
2. **次卡**：在售次卡商品列表、我的次卡切换入口。
3. **我的**：个人资料、我的订单、核销记录、协议、员工工作台入口。

### 6.2 页面清单

| 页面 | 路由建议 | 主要内容 | 角色 |
|---|---|---|---|
| 启动/登录 | `pages/auth/index` | 自动微信登录、失败重试 | 全部 |
| 首页 | `pages/home/index` | 问候、会员号、可用卡摘要、购买入口 | 全部 |
| 次卡商城 | `pages/card-store/index` | 在售商品、次数、有效期、价格 | 全部 |
| 次卡详情 | `pages/card-product/detail` | 商品详情、使用规则、购买按钮 | 全部 |
| 我的次卡 | `pages/my-cards/index` | 可用、已用完、已过期分类 | 全部 |
| 次卡详情 | `pages/my-cards/detail` | 剩余次数、有效期、核销记录、出示核销码 | 全部 |
| 核销码 | `pages/redemption-code/index` | 动态二维码、倒计时、自动刷新提示 | 全部 |
| 我的订单 | `pages/orders/index` | 订单列表和状态 | 全部 |
| 订单详情 | `pages/orders/detail` | 商品快照、金额、支付状态和时间 | 全部 |
| 核销记录 | `pages/redemptions/index` | 本人核销流水 | 全部 |
| 我的 | `pages/profile/index` | 会员资料、协议、员工入口 | 全部 |
| 员工工作台 | `pages/staff/index` | 扫码按钮、当日核销数据 | STAFF/ADMIN |
| 核销确认 | `pages/staff/redeem` | 核销预览、确认、结果 | STAFF/ADMIN |

### 6.4 Web 管理后台页面

| 页面 | 路由建议 | 主要内容 | 角色 |
|---|---|---|---|
| 登录 | `/login` | 用户名、密码、错误和锁定提示 | 公开 |
| 数据概览 | `/` | 会员数、在售商品、今日订单、今日核销 | ADMIN |
| 次卡商品 | `/card-products` | 新增、编辑、上下架、版本冲突提示 | ADMIN |
| 会员管理 | `/members` | 搜索会员，查看资料、次卡和流水 | ADMIN |
| 订单管理 | `/orders` | 查询订单、订单详情、全额退款 | ADMIN |
| 核销记录 | `/redemptions` | 查询全部核销流水和操作员工 | ADMIN |
| 员工管理 | `/staff` | 查询员工，授予或撤销 STAFF | ADMIN |
| 审计日志 | `/audit-logs` | 查询管理员操作和安全日志 | ADMIN |

Web 管理后台采用桌面优先的响应式布局。所有数据表格必须提供加载、空、错误、分页和筛选状态；退款、上下架、角色修改必须二次确认。

### 6.3 页面通用状态

所有列表和详情页必须实现：

- 首次加载状态；
- 空状态；
- 网络错误及重试；
- 无权限状态；
- 下拉刷新或显式刷新；
- 防止按钮重复提交；
- 统一 Toast/错误提示。

## 7. 状态机

### 7.1 次卡商品 `card_product.status`

```text
DRAFT -> ON_SALE -> OFF_SALE
            ^          |
            |----------|
```

- `DRAFT`：草稿，仅管理员可见。
- `ON_SALE`：在售，可创建订单。
- `OFF_SALE`：下架，不可创建新订单，不影响历史订单和已购卡。

### 7.2 订单 `order.status`

```text
PENDING -> PAID -> REFUNDING -> REFUNDED
   |                 |
   -> CLOSED         -> PAID（仅退款明确失败）
```

- `PENDING`：待支付。
- `PAID`：微信支付回调验证成功且已发卡。
- `CLOSED`：超时关闭或明确取消。
- `REFUNDING`：管理员已发起退款，等待微信退款最终结果。
- `REFUNDED`：微信确认全额退款成功。
- 待支付订单默认 30 分钟关闭。

### 7.3 用户次卡 `member_card.status`

```text
ACTIVE -> USED_UP
   |
   -> EXPIRED
   |
   -> REFUND_LOCKED -> REFUNDED
              |
              -> ACTIVE（仅退款明确失败）
```

- `ACTIVE`：已生效、未过期、剩余次数大于 0。
- `USED_UP`：剩余次数为 0。
- `EXPIRED`：到达过期时间。
- `REFUND_LOCKED`：退款处理中，禁止生成核销码和核销。
- `REFUNDED`：订单已全额退款，卡片永久不可用。
- 状态变更由核销事务和定时任务处理；接口读取时仍须实时校验时间，不能只依赖定时任务。

### 7.4 核销令牌 `redemption_token.status`

```text
ACTIVE -> USED
   |
   -> EXPIRED
   |
   -> REVOKED
```

## 8. 数据模型

所有业务表至少包含：`id`、`created_at`、`updated_at`。主键建议使用 UUIDv7 或应用生成的有序 64 位 ID，禁止依赖可枚举 ID 作为权限边界。

### 8.1 `users`

| 字段 | 类型建议 | 约束/说明 |
|---|---|---|
| `id` | bigint/char(36) | 主键 |
| `member_no` | varchar(32) | 唯一、可展示，如 `UR2026000001` |
| `nickname` | varchar(64) | 可空 |
| `avatar_url` | varchar(512) | 可空；V1 不提供上传 |
| `phone` | varchar(32) | 可空、加密或受控访问 |
| `role` | varchar(16) | MEMBER/STAFF/ADMIN |
| `status` | varchar(16) | ACTIVE/DISABLED |
| `last_login_at` | datetime(3) | 可空 |
| `version` | int | 乐观锁版本号，后台修改状态时递增 |

### 8.1.1 `member_profiles`

会员运营资料与登录主账号分表，避免未来教练、员工等角色字段污染 `users`。

| 字段 | 类型建议 | 约束/说明 |
|---|---|---|
| `user_id` | FK | 唯一关联 MEMBER 用户 |
| `phone_last4` | char(4) | 可空；仅用于后台按后四位精确搜索，完整手机号继续加密存储 |
| `phone_authorized_at` | datetime(3) | 用户主动授权手机号的时间 |
| `privacy_consent_version` | varchar(32) | 可空；用户同意的隐私政策版本 |
| `privacy_consent_at` | datetime(3) | 可空；隐私政策同意时间 |
| `marketing_consent_at` | datetime(3) | 可空；营销信息单独授权时间 |
| `tags` | json | 最多 10 个后台运营标签，不对会员展示 |
| `admin_note` | varchar(500) | 内部备注，不对会员展示 |

V1 不主动采集性别、生日、身份证号等非必要信息。完整手机号只对已登录管理员显示，传输使用 HTTPS、数据库保持加密存储；微信 OpenID/UnionID 仅用于身份关联，不在管理后台展示。

### 8.2 `wechat_identities`

| 字段 | 类型建议 | 约束/说明 |
|---|---|---|
| `user_id` | FK | 关联用户 |
| `appid` | varchar(64) | 小程序 AppID |
| `openid` | varchar(128) | 与 appid 组成唯一索引 |
| `unionid` | varchar(128) | 可空 |

严禁存储微信 `session_key` 的明文长期副本。

### 8.3 `card_products`

| 字段 | 类型建议 | 约束/说明 |
|---|---|---|
| `name` | varchar(100) | 商品名称 |
| `short_description` | varchar(255) | 商城卡片的一句话卖点 |
| `description` | text | 商品详情与适合人群 |
| `total_times` | int | 大于 0 |
| `validity_days` | int | 大于 0 |
| `activation_mode` | varchar(16) | PURCHASE/FIRST_USE |
| `price_cent` | bigint | 大于等于 1 |
| `list_price_cent` | bigint | 可空，不得低于售价 |
| `purchase_limit` | int | 可空，每位会员限购张数 |
| `daily_use_limit` | int | 每张卡每日最大核销次数 |
| `transferable` | boolean | 是否允许同行人共享次数 |
| `rules` | text | 使用限制、装备、活动和退款说明 |
| `badge` | varchar(32) | 可空，如推荐/新客专享 |
| `theme_color` | char(7) | 商城卡面主题色 |
| `status` | varchar(16) | DRAFT/ON_SALE/OFF_SALE |
| `sort_order` | int | 默认 0 |
| `version` | int | 乐观锁/编辑版本 |

### 8.4 `orders`

| 字段 | 类型建议 | 约束/说明 |
|---|---|---|
| `order_no` | varchar(64) | 全局唯一、对外展示 |
| `user_id` | FK | 下单会员 |
| `biz_type` | varchar(32) | V1 为 `CARD_PURCHASE`，为课程/商品预留 |
| `status` | varchar(16) | 订单状态 |
| `total_amount_cent` | bigint | 应付金额 |
| `paid_amount_cent` | bigint | 实付金额，未支付为 0 |
| `product_snapshot` | json | 商品 ID、名称、次数、有效期、单价快照 |
| `client_request_id` | varchar(64) | 用户侧幂等键，同用户唯一 |
| `expires_at` | datetime(3) | 支付过期时间 |
| `paid_at` | datetime(3) | 可空 |
| `closed_at` | datetime(3) | 可空 |

### 8.5 `payment_transactions`

| 字段 | 类型建议 | 约束/说明 |
|---|---|---|
| `order_id` | FK | 关联订单 |
| `provider` | varchar(16) | `WECHAT_PAY` |
| `merchant_order_no` | varchar(64) | 唯一 |
| `provider_transaction_id` | varchar(128) | 微信支付单号，支付后唯一 |
| `status` | varchar(16) | CREATED/SUCCEEDED/FAILED/CLOSED |
| `amount_cent` | bigint | 支付金额 |
| `callback_payload` | json/text | 脱敏后的回调审计信息 |
| `succeeded_at` | datetime(3) | 可空 |

### 8.6 `member_cards`

| 字段 | 类型建议 | 约束/说明 |
|---|---|---|
| `card_no` | varchar(64) | 全局唯一、可展示 |
| `user_id` | FK | 所属会员 |
| `source_order_id` | FK | 来源订单；V1 唯一，确保一单只发一张卡 |
| `product_id` | FK | 来源商品 |
| `product_name` | varchar(100) | 商品名称快照 |
| `total_times` | int | 初始总次数快照 |
| `remaining_times` | int | 0 到 total_times |
| `status` | varchar(24) | ACTIVE/USED_UP/EXPIRED/REFUND_LOCKED/REFUNDED |
| `activated_at` | datetime(3) | 生效时间 |
| `expires_at` | datetime(3) | 到期时间 |
| `version` | int | 并发控制 |

### 8.7 `redemption_tokens`

| 字段 | 类型建议 | 约束/说明 |
|---|---|---|
| `token_hash` | char(64) | 唯一；只存令牌哈希，不存原文 |
| `member_card_id` | FK | 目标卡 |
| `user_id` | FK | 卡片所有者 |
| `status` | varchar(16) | ACTIVE/USED/EXPIRED/REVOKED |
| `expires_at` | datetime(3) | 默认创建后 5 分钟 |
| `used_at` | datetime(3) | 可空 |

### 8.8 `redemption_records`

| 字段 | 类型建议 | 约束/说明 |
|---|---|---|
| `redemption_no` | varchar(64) | 全局唯一 |
| `member_card_id` | FK | 被核销卡 |
| `user_id` | FK | 会员 |
| `operator_user_id` | FK | 核销员工 |
| `token_id` | FK | 唯一，确保令牌仅核销一次 |
| `times` | int | V1 固定为 1 |
| `before_remaining` | int | 核销前次数 |
| `after_remaining` | int | 核销后次数 |
| `redeemed_at` | datetime(3) | 核销时间 |
| `request_id` | varchar(64) | 员工端幂等键，与 operator 组成唯一索引 |

### 8.9 `audit_logs`

记录管理员修改商品、角色，以及关键异常人工操作。字段至少包含：操作者、动作、资源类型、资源 ID、变更前后摘要、请求 IP、请求 ID、时间。

### 8.10 `refund_transactions`

| 字段 | 类型建议 | 约束/说明 |
|---|---|---|
| `refund_no` | varchar(64) | 系统退款单号，全局唯一 |
| `order_id` | FK | 关联原订单；V1 与订单一对一 |
| `payment_transaction_id` | FK | 关联原支付流水 |
| `provider_refund_id` | varchar(128) | 微信退款单号，可空，返回后唯一 |
| `status` | varchar(24) | CREATED/PROCESSING/SUCCEEDED/FAILED/ABNORMAL |
| `amount_cent` | bigint | 固定等于订单实付金额 |
| `reason` | varchar(255) | 管理员填写，必填 |
| `operator_user_id` | FK | 发起退款的 ADMIN |
| `client_request_id` | varchar(64) | 幂等键，与操作者组成唯一索引 |
| `callback_payload` | json/text | 脱敏后的退款通知审计信息 |
| `succeeded_at` | datetime(3) | 可空 |
| `failed_at` | datetime(3) | 可空 |

### 8.11 `admin_credentials`

| 字段 | 类型建议 | 约束/说明 |
|---|---|---|
| `user_id` | FK | 唯一；关联角色为 ADMIN 的用户 |
| `username` | varchar(64) | 唯一，统一转为小写存储 |
| `password_hash` | varchar(255) | Argon2id 或 bcrypt 哈希，禁止可逆加密 |
| `must_change_password` | boolean | 初始账号为 true |
| `failed_attempts` | int | 连续失败次数 |
| `locked_until` | datetime(3) | 可空 |
| `password_changed_at` | datetime(3) | 可空 |
| `last_login_at` | datetime(3) | 可空 |

### 8.12 `admin_sessions`

只存管理会话的随机令牌哈希，不存 Cookie 原文。字段至少包含：管理员用户、session token hash、CSRF token hash、过期时间、撤销时间、最后访问时间、IP 和 User-Agent 摘要。

## 9. API 契约

### 9.1 通用规则

- 前缀：`/api/v1`。
- Content-Type：`application/json`。
- 鉴权：`Authorization: Bearer <access_token>`。
- 每个请求接受或生成 `X-Request-ID`，并写入日志。
- 写操作支持 `Idempotency-Key`；支付下单和核销接口必须支持。
- 分页使用 `page`、`page_size`，默认 1/20，最大 100。
- 成功响应：`{"data": ..., "request_id": "..."}`。
- 失败响应：`{"error":{"code":"...","message":"..."},"request_id":"..."}`。
- `message` 可展示给用户时使用中文；`code` 必须稳定，前端按 code 处理。
- 不得在响应或日志中输出 `session_key`、支付密钥、完整 Authorization、手机号明文或核销令牌原文。

### 9.2 登录与用户

| 方法 | 路径 | 功能 | 权限 |
|---|---|---|---|
| POST | `/auth/wechat/login` | 使用 wx.login code 登录/注册 | 公开 |
| POST | `/auth/refresh` | 刷新访问令牌 | 刷新令牌 |
| POST | `/auth/logout` | 注销当前刷新令牌 | MEMBER |
| GET | `/me` | 当前会员信息和角色 | MEMBER |
| PUT | `/me` | 修改昵称等允许字段 | MEMBER |

### 9.2.1 Web 管理后台认证

| 方法 | 路径 | 功能 | 权限 |
|---|---|---|---|
| POST | `/admin/auth/login` | 管理员用户名密码登录，设置 HttpOnly Cookie | 公开/限流 |
| POST | `/admin/auth/logout` | 撤销当前管理会话 | ADMIN |
| POST | `/admin/auth/change-password` | 修改密码并撤销其他会话 | ADMIN |
| GET | `/admin/me` | 当前管理员资料、权限和 CSRF token | ADMIN |

### 9.3 次卡商品与会员卡

| 方法 | 路径 | 功能 | 权限 |
|---|---|---|---|
| GET | `/card-products` | 在售商品列表 | MEMBER |
| GET | `/card-products/{id}` | 商品详情 | MEMBER |
| GET | `/me/cards` | 我的次卡列表，可按状态筛选 | MEMBER |
| GET | `/me/cards/{id}` | 我的次卡详情 | MEMBER |
| GET | `/me/cards/{id}/redemptions` | 指定卡核销记录 | MEMBER |

### 9.4 订单与支付

| 方法 | 路径 | 功能 | 权限 |
|---|---|---|---|
| POST | `/orders` | 创建次卡订单 | MEMBER |
| GET | `/me/orders` | 我的订单列表 | MEMBER |
| GET | `/me/orders/{order_no}` | 我的订单详情 | MEMBER |
| POST | `/orders/{order_no}/wechat-pay` | 获取微信支付调起参数 | MEMBER |
| POST | `/payments/wechat/notify` | 微信支付回调 | 微信支付平台签名 |
| POST | `/orders/{order_no}/sync` | 主动查询微信支付并同步状态 | MEMBER |

`POST /orders` 请求必须包含 `card_product_id` 和客户端幂等键。后端不得接受前端传入的价格、次数或有效期作为可信值。

### 9.5 核销

| 方法 | 路径 | 功能 | 权限 |
|---|---|---|---|
| POST | `/me/cards/{id}/redemption-token` | 创建/刷新一次性核销令牌 | MEMBER |
| POST | `/staff/redemptions/preview` | 预览核销信息 | STAFF |
| POST | `/staff/redemptions/confirm` | 确认核销 | STAFF |
| GET | `/staff/redemptions/today` | 当日核销列表 | STAFF |

建议错误码：

- `CARD_NOT_FOUND`
- `CARD_NOT_OWNED`
- `CARD_EXPIRED`
- `CARD_USED_UP`
- `CARD_NOT_ACTIVE`
- `TOKEN_INVALID`
- `TOKEN_EXPIRED`
- `TOKEN_ALREADY_USED`
- `REDEMPTION_ALREADY_COMPLETED`
- `INSUFFICIENT_PERMISSION`

### 9.6 管理接口

| 方法 | 路径 | 功能 | 权限 |
|---|---|---|---|
| GET/POST | `/admin/card-products` | 查询/新建商品 | ADMIN |
| GET/PUT | `/admin/card-products/{id}` | 查看/编辑商品 | ADMIN |
| POST | `/admin/card-products/{id}/status` | 上架/下架（携带版本号） | ADMIN |
| GET | `/admin/members` | 搜索会员 | ADMIN |
| GET | `/admin/members/{id}` | 会员详情、卡和记录 | ADMIN |
| PUT | `/admin/members/{id}` | 修改会员状态、运营标签和内部备注（携带版本号） | ADMIN |
| GET | `/admin/orders` | 全部订单查询 | ADMIN |
| GET | `/admin/orders/{order_no}` | 订单、会员卡和退款详情 | ADMIN |
| POST | `/admin/orders/{order_no}/refund` | 发起整单全额退款 | ADMIN |
| POST | `/admin/refunds/{refund_no}/sync` | 主动查询并同步微信退款状态 | ADMIN |
| GET | `/admin/redemptions` | 全部核销查询 | ADMIN |
| GET | `/admin/staff` | 查询员工与可授权会员 | ADMIN |
| PUT | `/admin/users/{id}/role` | 设置 MEMBER/STAFF | ADMIN |
| GET | `/admin/audit-logs` | 查询不可变的关键操作审计日志 | ADMIN |

V1 禁止通过 API 创建第二个 ADMIN；初始 ADMIN 由部署配置或数据库迁移脚本明确指定。

## 10. 支付一致性要求

支付属于最高风险模块，必须满足：

1. 使用微信支付 API v3，商户私钥和 APIv3 密钥只从运行环境 Secret 读取。
2. 验证回调时间戳、随机串、签名和平台证书。
3. 验证回调中的商户号、AppID、订单号、币种和金额。
4. 回调可能重复、乱序或延迟；处理逻辑必须幂等。
5. `orders.order_no`、支付单号、微信交易号均设置唯一约束。
6. “订单置为 PAID”和“创建 member_card”在一个数据库事务中完成。
7. 已支付订单再次收到成功回调时直接返回成功，不重复发卡。
8. 前端支付成功只触发订单查询，不能直接发卡。
9. 保存足够的脱敏审计信息，但不要保存支付敏感明文。
10. 支付回调接口应快速响应；非关键通知任务不得阻塞回调。

### 10.1 退款一致性要求

1. 退款接口仅允许 `ADMIN` 调用，会员端不得暴露对应能力。
2. V1 退款金额由后端固定为原订单 `paid_amount_cent`，不得接受前端自定义金额。
3. 发起退款前，在数据库事务中锁定订单和会员卡，并确认卡片没有任何核销记录。
4. 订单进入 `REFUNDING` 时，会员卡必须同步进入 `REFUND_LOCKED`，防止退款与核销并发发生。
5. 退款请求必须使用系统退款单号和幂等键；重复请求返回原退款单。
6. 退款最终成功以验签通过的微信退款通知或主动查询结果为准，不能以管理端调用返回为准。
7. 验证退款通知中的商户号、订单号、退款单号、币种和金额。
8. 退款通知可重复、乱序或延迟；处理必须幂等，且一笔订单最多产生一笔成功退款。
9. 未知、处理中或异常状态不得恢复会员卡；只有微信明确返回退款失败时才可恢复为原状态。
10. 发起人、原因、状态变化和微信结果必须写入退款流水及管理员审计日志。

## 11. 核销一致性要求

核销必须使用数据库事务，建议顺序：

1. 按令牌哈希查询并 `SELECT ... FOR UPDATE` 锁定令牌。
2. 若 request id 已存在，返回原核销记录。
3. 校验令牌状态和有效期。
4. 锁定对应 `member_cards` 行。
5. 再次校验卡状态、过期时间和 `remaining_times > 0`。
6. 将剩余次数减 1，并在为 0 时置为 `USED_UP`。
7. 写入不可变的 `redemption_records`。
8. 将令牌置为 `USED`。
9. 提交事务后返回结果。

严禁采用“先查询余额，再无条件 update”的实现。必须同时使用事务、行锁/条件更新和唯一约束防止重复扣减。

## 12. Go 后端技术要求

### 12.1 架构

采用模块化单体，不使用微服务。推荐目录：

```text
backend/
  cmd/api/                 # 程序入口
  internal/
    auth/
    member/
    card/
    order/
    payment/
    redemption/
    admin/
    platform/wechat/       # 微信登录与支付适配器
    middleware/
    config/
  migrations/
  pkg/                     # 仅真正可复用的公共包
  tests/
```

每个业务模块至少区分：

- HTTP handler：参数解析、鉴权上下文、响应格式；
- application/service：用例编排和事务边界；
- domain：业务规则和状态转换；
- repository：数据库访问；
- DTO：禁止直接将数据库模型暴露给 API。

### 12.2 实现约束

- 使用稳定版 Go，并在 `go.mod` 固定版本。
- HTTP 框架可选 Gin 或标准库，但全项目统一。
- 数据访问可选 `sqlc`、GORM 或 `database/sql`；涉及支付和核销的事务必须显式、可审查。
- 配置使用环境变量，提供 `.env.example`，不得提交真实密钥。
- 使用结构化 JSON 日志。
- 数据库变更必须使用版本化 migration，禁止启动时自动修改生产表结构。
- 为服务实现 `/health/live` 和 `/health/ready`。
- 实现优雅停机、HTTP 超时、数据库连接池和 panic recovery。
- 所有外部微信请求设置超时和有限重试；支付创建接口不得盲目重试产生重复订单。
- API 文档使用 OpenAPI 3，并与实现同步。
- 业务错误与 HTTP 状态码建立统一映射。

## 13. 微信小程序技术要求

- 使用微信原生小程序技术栈：WXML、WXSS、JavaScript 或 TypeScript；优先 TypeScript。
- 启用基础库兼容性配置，并明确最低基础库版本。
- 环境分为 `development`、`trial`、`production`，API 地址不得散落在页面代码中。
- 请求层统一处理 token、刷新、request id、错误码和超时。
- access token 只存于微信安全存储能力允许的位置；退出时清除。
- 页面不得直接拼接底层 API；通过 `services/` 层访问。
- 组件至少包含：加载态、空状态、错误态、金额展示、状态标签、确认弹窗。
- 扫码只负责取得令牌，所有核销判断在后端完成。
- 管理员/员工入口依据 `/me` 返回角色显示，但后端仍独立鉴权。
- 金额统一由分转换为展示字符串，不使用浮点运算参与业务。
- 小程序代码不得包含微信支付商户密钥、AppSecret 或任何服务器 Secret。

建议目录：

```text
miniprogram/
  pages/
  components/
  services/
  store/
  utils/
  config/
  types/
  assets/
```

## 13.1 Web 管理后台技术要求

- 使用 React、TypeScript、Vite 和 React Router，采用桌面优先的响应式布局。
- 代码独立放在 `admin-web/`，按 `pages/`、`components/`、`services/`、`layouts/`、`types/` 组织。
- API 统一使用同源 `/api/v1`，生产环境由 Nginx 反向代理，不在代码中硬编码服务器 IP。
- 所有请求带 `credentials: include`；管理会话只使用 HttpOnly Cookie，禁止写入 localStorage/sessionStorage。
- 修改类请求必须带后端签发的 CSRF token 和 `Idempotency-Key`（适用时）。
- 路由守卫只能改善体验，后端必须独立校验 ADMIN 权限。
- 表格支持服务端分页、筛选和稳定排序；金额仍以整数分传输。
- 退款、上下架、角色修改等危险操作必须使用明确的二次确认弹窗，按钮防重复提交。
- 统一处理 401、403、409、422、429 和 5xx；401 跳转登录，409 显示版本冲突并刷新数据。
- 不引入大而全的状态管理框架；在出现明确跨页复杂状态前使用 React Context 和页面请求状态。
- 构建产物为静态文件，由 Nginx 托管；开发环境通过 Vite proxy 访问 Go API。

## 14. 安全与隐私

1. 所有生产接口只允许 HTTPS。
2. 小程序访问令牌短有效期，刷新令牌可撤销并支持轮换；Web 管理会话使用 HttpOnly Cookie、CSRF 防护和服务端撤销。
3. 小程序登录、管理员登录、支付创建、核销码生成、扫码预览和核销确认必须限流。
4. 所有资源查询必须校验所有权，禁止仅凭 ID 返回他人数据。
5. 管理接口进行 RBAC 校验并记录审计日志。
6. 数据库不开放公网端口；服务器防火墙只开放 22（限制来源）、80、443。
7. MySQL 使用独立低权限账号，应用账号无建库和授权权限。
8. 日志脱敏手机号、openid、token、支付信息。
9. 核销令牌至少 128 bit 随机熵，仅在数据库存 SHA-256 哈希。
10. 用户协议和隐私政策中说明收集目的、范围和保存方式。
11. 提供账号停用能力的后台基础字段；账号注销流程可在 V1 先通过客服联系，但须在协议中说明。

## 15. 运维与部署

### 15.1 Docker Compose 服务

```text
nginx
api
mysql
backup
admin-web（静态构建产物由 nginx 托管，不单独常驻进程）
```

- Nginx 终止 TLS 并反向代理 API。
- MySQL 数据使用独立 Docker volume。
- 每日执行逻辑备份，并传输到服务器外的受控备份位置；当前不使用 COS 时，至少同步到开发者持有的另一台设备或其他独立存储。仅保存在同一服务器不算备份。
- 保留最近 30 天每日备份和最近 6 个月月度备份。
- 应用日志滚动保留 14 天，避免占满磁盘。
- 配置 CPU、内存、磁盘使用率、HTTPS 到期和 API 存活告警。

### 15.2 环境

- 本地开发：Docker Compose，微信支付使用 mock provider 或沙箱式适配器。
- 体验环境：允许与生产部署在同一服务器，但必须使用独立数据库、独立配置和独立回调路径。
- 生产环境：生产数据库和测试数据库严格隔离。
- 微信支付真实回调只允许生产配置；测试不得创建真实会员卡。

## 16. 测试要求

### 16.1 后端自动化测试

必须覆盖：

- 首次微信登录和重复登录；
- 禁用用户登录；
- 商品上下架与下架商品禁止下单；
- 创建订单幂等；
- 支付回调验签失败；
- 支付金额不一致；
- 重复支付回调只发一张卡；
- 普通会员和 STAFF 无法调用退款接口；
- 已核销次卡、非 PAID 订单和重复退款请求被正确拒绝或幂等返回；
- 退款发起后卡片立即不可核销；
- 重复退款通知只处理一次，退款成功后卡片永久不可用；
- 退款明确失败后订单与卡片状态正确恢复；
- 用户不能读取他人的订单和卡；
- 过期卡、用尽卡禁止生成核销码；
- 核销码过期、撤销、重复使用；
- 10 个并发核销请求只能成功一次；
- STAFF 和 ADMIN 权限边界；
- 商品修改不影响历史订单和卡片快照。

支付回调、发卡和核销必须有集成测试，且使用真实 MySQL 测试容器验证事务与唯一约束，不能仅依赖 mock repository。

### 16.2 小程序验收

- 真机测试微信登录、支付调起和扫码。
- 弱网、断网、超时、重复点击均有明确反馈。
- iOS 与 Android 微信最新版完成核心流程回归。
- 员工账号和普通会员账号分别验收入口与权限。
- 核销成功后，会员端刷新能看到次数变化和流水。

## 17. 验收标准（Definition of Done）

一个功能只有同时满足以下条件才算完成：

1. 与本 PRD 的业务规则一致。
2. API、数据库 migration 和 OpenAPI 文档同步。
3. 有成功、失败、空状态和权限错误处理。
4. 核心逻辑具备自动化测试。
5. `go test ./...`、静态检查和小程序构建通过。
6. 不包含真实 AppSecret、商户私钥、数据库密码。
7. 日志中无敏感信息和核销令牌原文。
8. 在体验版真机完成相应流程。
9. 代码经过 AI Agent 自检，支付、权限、事务、并发相关变更需单独列出风险说明。

## 18. AI Agent 执行顺序

AI Agent 应按以下里程碑实施，每个里程碑完成后测试并提交独立 commit：

### M0：工程基础

- 初始化 Go 后端、微信小程序、React Web 管理后台、Docker Compose。
- 配置管理、日志、统一错误、request id、健康检查。
- MySQL migration 框架和 OpenAPI 基础。
- CI：Go 测试、静态检查、小程序类型检查。

### M1：登录与会员

- 微信登录适配器及本地 mock。
- 用户、微信身份、token、角色权限。
- `/me` 和个人资料页面。
- Web 管理员凭据、Cookie 会话、CSRF 和登录页面。
- 初始化首个 ADMIN 的安全 CLI 流程。

### M2：次卡商品与管理

- 商品表、状态机、管理 API。
- 小程序商品列表与详情，以及 Web 管理后台商品管理页。
- 商品上下架和审计日志。

### M3：订单与微信支付

- 订单、支付交易、商品快照。
- 下单幂等、微信支付参数、回调验签。
- 支付事务发卡、订单列表和详情。
- 管理员整单退款、退款通知验签、退款状态同步和卡片锁定。

### M4：会员卡与核销

- 我的次卡和详情。
- 一次性核销令牌。
- 员工扫码预览和确认。
- 并发核销测试与核销记录。

### M5：上线准备

- 隐私政策和用户协议入口。
- 生产 Docker Compose、Nginx、HTTPS、备份脚本说明。
- 告警、日志滚动、数据库恢复演练。
- 体验版全流程验收与生产配置清单。

## 19. AI Agent 工作约束

1. 开始编码前先读取本 PRD，并输出当前里程碑、任务清单、受影响模块和风险。
2. 一次只实现一个里程碑或一个可验收的纵向功能，不做无关重构。
3. 遇到 PRD 未定义且会影响金额、有效期、权限、退款、核销或数据删除的事项，必须暂停并向产品负责人提问，不得自行假设。
4. 普通技术细节可采用最简单、稳定且可测试的实现，并在变更说明中记录决定。
5. 不得通过跳过验签、关闭鉴权、硬编码管理员、使用固定核销码等方式完成演示。
6. 不得为了“可扩展”提前拆微服务、引入消息队列、Redis、Kubernetes 或复杂领域框架。
7. 每次交付必须说明：已完成、未完成、测试结果、数据库变更、配置变更、已知风险。

## 20. 上线前待产品负责人提供

以下信息不阻塞 M0-M2，但会阻塞真实支付和上线：

- 微信小程序 AppID；
- 小程序 AppSecret（只能配置在服务端 Secret，不写入文档或代码）；
- 微信支付商户号；
- API v3 密钥、商户证书序列号和商户私钥；
- 品牌 Logo、主色和页面视觉规范；
- 首批次卡名称、次数、售价、有效期及使用规则；
- 首个管理员微信账号对应的 openid，或一次性初始化方案；
- 客服联系方式；
- 用户协议、隐私政策、退款规则最终文本；
- 域名备案完成信息和服务器登录/部署方式。

## 21. 仍需业务确认但不阻塞工程初始化

1. 同一用户是否允许同时拥有多张相同次卡：V1 默认允许。
2. 到期当天的显示文案：实现以精确到秒的 `expires_at` 为准。
3. 员工核销是否需要选择门店：V1 单门店，不记录门店字段；若预计近期多店，应在编码 M4 前确认。
4. 会员是否必须授权手机号后才能购卡：V1 默认不强制。
5. 未支付订单是否允许用户主动取消：V1 默认仅自动关闭。
