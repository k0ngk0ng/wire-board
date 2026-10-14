# 连连看迁移与验收

迁移来源为同级 `wire-pairs`，基准提交为 `9c7ece32d9544daa35aa2d52aa044675cb64180c`，不改动源仓库。复用 51 个图标、5 帧闪电及原始音效，保留最多两次转弯、边界绕行、石块、木箱、冰封、提示、重排、破障锤和无解免费重排。账号、房间、存档、静音和托管统一由游戏平台维护。

对战采用 1～6 人同时消除：每局随机抽取地图与难度，每位玩家使用同一初始棋盘，各自消除。3 秒倒计时结束后开放棋盘，服务端首先确认清空的玩家获胜；其余玩家并列第二。单人是相同随机题面的计时练习。没有攻击、抢分或按剩余块数提前判胜；原单机的关卡积分与倒计时不决定竞速胜负。连击作为视觉反馈保留。提示 3 次、重排 2 次、破障锤 2 次，每人独立；无解自动重排不消耗道具。120 秒无有效操作自动托管，取消托管仅恢复本人操作期限。

## 实现前失败清单

- 连线穿过实体、超过两次转弯、越过一格外边界、不同图案或冰封图案被消除。
- 木箱未破除、冰层未解冻、锤子破坏普通图标或扣错道具；重排丢失图案、奇数配对或无解卡死。
- 随机图只有表面可消除、完整证明失败；不同玩家获得不同难度或共用可变棋盘。
- 开始前偷看棋盘；泄露随机种子／生成答案；客户端提交胜负、积分、时间或任意棋盘。
- 多人同时操作互相造成全局版本冲突；本人的旧棋盘请求或重复请求被再次执行。
- 胜者由完成百分比决定；第二个完成请求覆盖首胜；电脑或托管绕过合法操作。
- 重连／SQLite 恢复重置图案、工具、开始时间或其他玩家的期限；观众可操作。
- 新游戏默认上架；创建、等待、玩家栏另起布局；手机图标过小或无法点击；缺失键盘操作。
- 图片或音效仍请求原 `/llk/` 路径；素材漏传、缺少来源／校验；OSS 失败后无法使用本地回退。
- 重复播放旧特效、跳过连接动画、静音无效或直接创建游戏播放器。

## 验证安排

先编写连线／生成器隔离测试，再迁移算法。使用真实 HTTP、认证会话与 SQLite 验证竞速、并发、观战、默认下架与恢复；浏览器检查 PC、320px 手机、图标点击、特效、音效、素材回退和完整通关。证据保存到仓库 `.local/pairs/` 及现有 HTTP 测试记录目录。

素材来自公开复刻项目，不宣称为腾讯官方开放资源或已确认 CC0；来源与许可状态随原项目证据记录。新 OSS 前缀固定为 `bga/v20261002-1/pairs/v1/`，本地回退为 `/pairs/v1/`，不引用旧 OSS 路径。

## 素材与平台接入

`server/` 独立维护连线、生成、竞速、道具和电脑策略；`web/` 提供棋盘、规则、封面和公共玩家栏中的消除进度。分类为「益智休闲」。默认下架，由管理员决定上架；不会因发布或重启自动上架。

`assets/source/` 保存原始图标、闪电和音频；`source-manifest.json` 保存来源清单；`manifest.json` 保存 67 个成品的路径与源文件／成品 SHA-256。51 张图标和 5 帧闪电转为无损 WebP，10 个音频保持原始字节，新增封面为独立 SVG 插画并生成 WebP。背景音乐使用原项目选定的 `bg-legacy-2015.mp3`。背景音乐与音效都经平台原生媒体通道，统一静音；不会检测手机铃声静音开关。

重新准备素材（所有写入都在当前仓库，`cwebp` 与已有 resvg 依赖需可用）：

```sh
. scripts/env.sh
python3 -B scripts/pairs/prepare-assets.py
node scripts/pairs/render-cover.cjs
python3 -B scripts/pairs/verify-assets.py
```

`publish-assets.py` 使用仓库已忽略的 OSS 凭据。先比对已有平台素材以确认 bucket，再核对／上传新路径；上传带 `x-oss-forbid-overwrite`，已有同路径仅允许内容完全一致，随后回读 OSS 与 CDN 校验。不得用覆盖对象的方式发布修改后的素材。

## 可重复验收

```sh
. scripts/env.sh
go test -count=1 -timeout=6m ./games/pairs/...
node scripts/pairs/verify-source.mjs
node --experimental-strip-types --test web/tests/platform-audio.test.mjs web/tests/computer-support.test.mjs
WIRE_BOARD_PAIRS_CASE=oss WIRE_BOARD_PAIRS_ASSETS=https://cdn.ichenj.com/bga/v20261002-1 python3 -B scripts/pairs/accept-ui.py
WIRE_BOARD_PAIRS_CASE=fallback WIRE_BOARD_PAIRS_ASSETS=https://cdn.ichenj.com/bga/v20261002-1/missing-pairs-assets python3 -B scripts/pairs/accept-ui.py
python3 -B scripts/pairs/verify-assets.py --dist --oss
```

`verify-source.mjs` 比对 70 个原始关卡、每关 3 个种子，共 210 个样本的棋盘和重排结果，并用原始 TypeScript 消除序列验证 Go 生成的棋盘。`source-engine.ts` 仅作为独立验收基准，不进入产品构建。

HTTP 验收通过真实账号、房间、SQLite 和系统时钟运行；覆盖 1～6 人自然完整竞速、同版本多人操作、重复请求、观战只读、道具、真实电脑调度与接管、真实 120 秒超时托管。石块／木箱／冰封道具验收是明确选择原生生成结果的定向夹具，不计入自然完整对局数。

浏览器验收使用真实点击、CDP 原生触屏与键盘，通过原始引擎寻找可操作的配对，不注入游戏状态；覆盖手机 320px、桌面 1280px、同步时保留选中图案、音频静音和本地素材回退。截图及 JSON 在 `.local/pairs/browser-*/`，素材证据在 `.local/pairs/oss-readback.json` 和 `.local/pairs/oss-verify.json`；HTTP 对局记录沿用公共测试工具，位于 `.local/poker/pairs/http/`。

main 工作流增加素材打包检查和连连看规则／HTTP 验收；版本 tag 继续只发布同一 SHA 已验证的构建。
