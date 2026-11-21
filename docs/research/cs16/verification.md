# CS 1.6 本地验收记录

规则、服务端、前端构建、定向回归及正式四图浏览器验收均已通过。本次完成本地实现与验证，未部署或发布。

## 交付范围

- 默认 10 席、5 对 5，支持 2／4／6／8／10 人、电脑、主动托管和观战。
- 24 种枪械、小刀、HE、闪光、烟雾、C4，以及护甲、拆弹器和弹药。
- `de_dust2`、`de_inferno`、`de_aztec` 三张爆破图，以及 `cs_assault` 人质营救图。
- 房主在创建或等待房间设置地图、回合时间、比赛局数和队友误伤；误伤默认关闭，开局固定。
- PC 保留经典键鼠操作；手机采用左指移动、右指开火并拖动瞄准，支持切换蹲姿和触摸购枪滚动。
- Three.js 绘制原图和转换后的模型；同源 Go／WASM 预测本人运动，服务器裁定伤害、经济、目标和胜负。

## 已通过的检查

以下路径以仓库根目录为基准。测试结果保存在 Git 忽略的 `.local/cs16/`，原始 JSON／日志可复核。

| 范围 | 结果 | 证据 |
| --- | --- | --- |
| 引擎与误伤 | 87 个普通测试通过事件，34 个误伤／战斗 race 测试通过事件 | `.local/cs16/verification/engine-ff-summary.json` |
| 真实十连接输入 | 10 个登录 WebSocket，1000 条输入全部确认，849 条／秒，ACK P95 95.806 ms | `.local/cs16/verification/server/capacity-deferred-summary.json` |
| 多房慢端隔离 | 4 房各 1 真人＋9 电脑并带慢观战，ACK P95 4.156 ms，规则落后 P95 36 ms | 同上 |
| 模块与完整比赛 | 服务端、注册、误伤、普通与批处理等价及四图自然十席比赛通过 | `.local/cs16/verification/server/ff-final-summary.json` |
| 前端与运动核心 | 65 项 Node 检查通过，四图共 960 帧原生／WASM 运动一致 | `.local/cs16/verification/frontend-final.log`、`motion-runtime.json` |
| 构建 | TypeScript 与 Vite 构建通过 | `.local/cs16/verification/web-build-final.log` |
| 代码规范 | 最终 139 个登记源码文件通过；共享 Go 正负例另有 40 项，lint 测试 12 项 | `.local/cs16/verification/lint/final-source.log`、`shared-go-summary.json` |
| 公共实时兼容 | 26 个既有实时批处理、队列、检查点、恢复、游标、加载与持久化测试通过事件 | `.local/cs16/verification/realtime-regression/summary.json` |
| Deferred 输入契约 | 普通与 race 各 12 个通过事件 | `.local/cs16/verification/deferred-input/summary.json` |
| 鉴权 | 会话缓存普通／race 与 24 项既有账号、管理员和 WebSocket 权限回归通过 | `.local/cs16/verification/auth-cache/summary.json`、`auth-regression/summary.json` |
| 资源发布工具 | 46 项 gzip／资源发布检查通过；无外部上传或下载请求 | `.local/cs16/verification/cdn/summary.json` |
| 资源登记 | 994 项、57,428,425 字节，清单及本地正文已校验，未上传 | `.local/cs16/verification/resources-registration.json` |
| 最终本地资源包 | CS16 素材与运行时共 997 项、60,798,906 字节，通过本地库存和摘要核对 | `.local/cs16/verification/cdn/final-built-inventory.json` |

ACK 是传输确认，不能代替按键到画面的延迟。四房结果也不代表 40 名真人同时高频输入。规则比赛、正式浏览器和服务端负载各自保留验收边界。

## 正式浏览器与来源绑定

正式命令为 `python3 -B games/cs16/tests/accept_browser.py`，不启用诊断、地图过滤或跳过完整比赛选项。最终运行退出码为 0，`passed:true`，功能和性能失败列表均为空。完整证据为 `.local/cs16/browser/reports/formal-fifth/report.json`，汇总为同目录 `verification-summary.json`，并保留原生事件、逐帧 CSV 和截图。

| 地图 | 原生事件到像素响应 P95 | 帧间隔 P99 | 显示帧率 |
| --- | --- | --- | --- |
| de_dust2 | 键盘 16 ms，鼠标 12.9 ms | 16.8 ms | 约 60 FPS |
| de_inferno | 键盘 16.4 ms，鼠标 14 ms | 16.8 ms | 约 60 FPS |
| de_aztec | 触屏 50 ms | 16.8 ms | 约 60 FPS |
| cs_assault | 键盘 17 ms，鼠标 15.3 ms | 16.8 ms | 约 60 FPS |

触屏结果恰好达到 50 ms 阈值，没有余量。前次正式运行测得 50.2 ms 并失败，同负载短诊断为 49.6 ms；这两份记录分别保存在 `formal-fourth` 和 `diagnostic-touch-timing`，不以最终单次通过代替波动说明。原生事件排队、GPU 像素读回与主线程观测均在原始报告中，总延迟和原门槛未作扣减或放宽。

正式页面还覆盖默认十席创建、房主误伤设置、经典键鼠、手机双指移动与开火拖动、独立松指、切换蹲姿、原生购枪滚动、横竖屏及两种全屏、快慢端并存、断线恢复和后台输入释放。手机视区与截图尺寸一致，竖屏为 320 × 760，横屏为 760 × 360。后台取消输入实测约 0.2 ms；恢复保持同局身份和已就绪画布，没有自动托管真人。

仓库突击以 2 个真人席位主动托管＋8 个电脑自然完成两回合，比分为 2:0。密集观察持续 14.038 秒，包含 3 名射手的 10 次开火；正常、密集与恢复包均未发现内部字段泄露，也未重发地图正文。

正常四图动态投影约 33.18～33.49 次／秒，应用消息正文 P95 为 4,611～4,900 字节；密集观战 P95 为 9,246 字节，恢复观察最大包为 14,708 字节。这些是 WebSocket 应用正文，不包含 TCP／TLS／帧头。

最终 231 个源码文件和 16 项构建产物开始、结束一致，运行时源码不匹配列表为空。浏览器源码身份为 `aff311ec3e9e7ad40f3d1580c6c7fb68213dd98f7f5f25b446f9cba7d6e0a9f0`。实际 Go 测试服务二进制 SHA-256 为 `200e008b1c9118f390eb6de979d35b3cf0bbfc5aed2564f1975d6841d8f2410b`，在运行时记录；临时可执行文件随测试正常退出而清除。

WASM 运行时构建摘要为 `56d4ae7e0884ca1fad269ee337fe86fc83b6fbba68f6301f20fc52575375354d`；运动核心源码摘要为 `745e1b4b2016afc7939f79e3657b3eccc17ee5088c253da29a1ca223cfd5ec9c`。正式浏览器报告另绑定实际 JS／CSS、入口、资源清单、WASM、公共传输源码和测试服务二进制，并要求开始与结束一致。

同步实现与限定证据见[即时同步审计](realtime-audit.md)，复跑入口见[游戏说明](../../../games/cs16/README.md)。

## 验收边界

原版素材来自固定摘要的第三方公开归档，参考源码另行固定提交。REA 完成二进制静态身份清点，没有运行或反编译原版；定点运动、随机数和命中盒存在已登记差异，不声称逐位等价。

触屏验收使用 Chromium 原生模拟，未以实体手机或手柄实测。此次仅在本地实现与验证，没有提交、上传 CDN、发布或更新服务器。

## 交付清理

本次创建的 Chrome、采集进程和 Go 测试服务已退出。已清理 `.local/cs16/` 内的 Go／Node／浏览器缓存、临时构建、过期运行时、未登记的未压缩 GLB 中间文件及已被取代的诊断截图和日志。保留原始研究样本、登记素材、当前运行时、最初失败证据、正式完整报告和触屏边界测量。未清理其他游戏或工作区外路径。

清理清单为 `.local/cs16/verification/cleanup.json`，最终机器汇总为 `.local/cs16/verification/final-summary.json`。
