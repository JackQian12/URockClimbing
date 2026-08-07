# URock 小程序首页 Design QA

## 验证对象

- 视觉参考：`/Users/jack/.codex/generated_images/019fc0bb-0f42-7210-b32e-5111bd49553e/exec-0d24752f-d6ad-4f43-bfde-38f6a1fbee6e.png`
- 实现截图：`miniprogram/qa/home-implementation.png`
- 并排对照：`miniprogram/qa/home-comparison.png`
- 源参考尺寸：853 × 1844
- 实现环境：微信开发者工具 Stable 2.01.2510290，iPhone 15 Pro Max 模拟器，基础库 3.15.2
- 对照尺寸：两侧归一化为 292 × 618

## 视觉结果

- 已对齐暖白底色、棕色品牌色、顶部 Logo 与问候语、会员次数层级、有效期、全宽核销 CTA、动态卡片、最近到店行与底部 TabBar。
- 首轮对照发现核销 CTA 宽度不足，已改为全宽可点击控件并重新编译截图。
- 实现图中的 iOS 状态栏、小程序胶囊和 Home Indicator 属于平台原生容器，参考图未绘制；不影响页面布局和视觉一致性判定。
- P0 问题：0；P1 问题：0；P2 问题：0。

## 功能与开发环境验证

- `npm run typecheck`：通过。
- `npm run qa:home`：通过，验证首页渲染、会员次数、购卡入口和核销入口存在。
- 微信开发者工具实际点击：“购买次卡”正确切换至 `pages/cards/index`；“生成核销码”正确展示登录/核销提示弹窗；首页、次卡、我的 Tab 正常显示。
- `npm audit --omit=dev`：0 个生产依赖漏洞。
- `make check`：通过，包含 Go 格式、race 测试、vet、小程序 TypeScript 检查和管理后台构建。
- 本地 Go API 未启动时，次卡页会显示网络错误；游客 AppID 下 `wx.login` 为开发者工具模拟返回。这两项是运行环境状态，不是前端缺少依赖。

final result: passed
