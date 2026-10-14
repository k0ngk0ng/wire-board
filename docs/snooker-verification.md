# 斯诺克验证记录

实现范围为纯 2D 俯视、标准十五红球、1v1 与 2v2，包含统一房间、物理与裁判、素材及平台音效。自动裁判的判定方式和边界见 [模块说明](../games/snooker/README.md)。

## 本地证据

| 验证 | 范围 | 产物 |
| --- | --- | --- |
| 物理与轨迹 | 连续碰撞、高速不穿透、高低杆／侧旋、袋口、贴球离开、确定性、无效输入拒绝；每次碰撞有准确时间的关键帧；稀疏轨迹重建不改变坐标 | `.local/snooker/verification/physics-green.log`、`contact-frames-green.log`、`sparse-physics-green.log`、轨迹 JSON |
| 裁判边界 | 红彩交替、最后红球、多红同进、罚分最高值、自由球、有效遮挡球、D 区狭窄可见区域、隔库路线验证、贴球回碰、占点复位、三次未中警告、黑球平分 | `.local/snooker/verification/go-final.log`、`referee-visibility-green.log` |
| 完整自然比赛（模块） | 单打单局赛 51 杆；双打三局两胜 153 杆，实际打满三局，结果 2:1。只从公开球位计算候选并提交真实击球参数，使用真实物理和裁判，推进模块时钟 | `.local/snooker/verification/TestSnookerNaturalRackToFinishedMatch/`、`natural-series.log` |
| 完整自然比赛（HTTP） | 单打、四人双打各一场单局赛，各 51 杆；真实注册、HTTP、系统计时及中途重启，不加速时钟。双打额外使用 1／2 号与 3／4 号分组 | `.local/snooker/verification/TestSnookerCompleteNaturalMatchesHTTP/`、`complete-http.log`、`.local/poker/snooker/http/` |
| 权限与持久化 | 默认下架、观众／其他席位拒绝击球、重复 nonce、旧操作拒绝、滚动中重启保留轨迹、一次性结算、原位恢复保留罚分、存档完整性校验 | `.local/snooker/verification/http.log`、`go-final.log` |
| 队伍与同意流程 | 双打整次上手轮换、原犯规者重打、下一局交替开球和换序；同版本并发准备／弃赛确认；本局让局须对手接受，黑球领先声明 | `.local/snooker/verification/consent-green.log`、`concessions-green.log`、`touch-and-order.log` |
| 真实浏览器 | 22 个检查点：公共创建／等待、2／4 人、全部席位、1280／390／320px、鼠标滚轮、拖动、原生触屏双指和瞄准、静音、原位恢复、实际碰撞音频播放、计算样式对比度 | `.local/snooker/browser/result.json`、同目录截图和 `*-contrast.json` |
| 定点反馈浏览器 | 明确球位夹具验证蓝球进袋后才加分、复位提示和重载不重播；真实隐藏／恢复标签页不补播落袋声；减少动态效果、实际媒体解码和坏 CDN 回退；接受认输本局 | `.local/snooker/feedback/colour*/result.json`、`concede/result.json` 和截图 |
| 素材 | 10 个图像、22 个 MP3；32 个 OSS 对象逐个回读，SHA-256、MIME 与格式签名一致；MP3 解码后非静音、无削波 | `.local/snooker/verification/oss-readback.json`、`audio-levels.json`、`local-assets.json` |
| 平台回归 | 319 项前端测试；模块生命周期／恢复针对性回归、Go vet、格式和游戏依赖边界 | `.local/snooker/verification/web-tests-final.log`、`platform-final.log`、`vet-final.log`、`boundaries-final.log` |

自然比赛共包含 6 局：模块单打 1 局、模块双打 3 局，以及 HTTP 单打、双打各 1 局。32 个满力度／旋转开球压力场景和其他明确球位夹具不计作完整局。HTTP 完整比赛按真实时间播放，约需 8～9 分钟；不把模拟时钟测试描述为实时比赛。

## 重复执行

从仓库根目录执行，所有缓存、账号、数据库和浏览器配置留在 `.local/`：

```sh
. scripts/env.sh
npm --prefix web run build
npm --prefix web test
go test -count=1 -timeout=5m ./games/snooker/...
WIRE_BOARD_SNOOKER_HTTP_FULL=1 go test -count=1 -timeout=12m -v -run '^TestSnookerCompleteNaturalMatchesHTTP$' ./games/snooker/tests
WIRE_BOARD_SNOOKER_ASSETS=https://cdn.ichenj.com/bga/v20261002-1 python3 -B scripts/accept_snooker.py
WIRE_BOARD_SNOOKER_ASSETS=https://cdn.ichenj.com/bga/v20261002-1 python3 -B scripts/snooker/check-feedback.py
WIRE_BOARD_SNOOKER_ASSETS=https://cdn.ichenj.com/bga/v20261002-1 WIRE_BOARD_SNOOKER_HIDDEN=1 python3 -B scripts/snooker/check-feedback.py
WIRE_BOARD_SNOOKER_ASSETS=https://cdn.ichenj.com/bga/v20261002-1 WIRE_BOARD_SNOOKER_REDUCED=1 python3 -B scripts/snooker/check-feedback.py
WIRE_BOARD_SNOOKER_CASE=concede python3 -B scripts/snooker/check-feedback.py
WIRE_BOARD_SNOOKER_FALLBACK=1 python3 -B scripts/snooker/check-feedback.py
python3 -B scripts/snooker/verify-assets.py --oss --dist
python3 -B scripts/snooker/verify-assets.py --decode-audio
python3 -B scripts/check_game_boundaries.py
```

浏览器命令须顺序执行，因为它们使用同一个测试服务地址文件。采用 `agent-browser`；原生滚轮、触摸和标签页可见性脚本仅连接该工具拥有的会话。明确球位夹具只编译进测试程序，不提供生产调试接口。

## main 与发布

main 的 `Test and Linux amd64 binary` 作业运行斯诺克规则和真实时间的完整 HTTP 比赛，并上传 `snooker-game-acceptance` 产物。其他规则分片不设置完整 HTTP 验收开关，避免重复等待同一批实时比赛。正式 tag 仅复用同一 SHA 已验证的二进制，不重复编译应用或运行检查矩阵。

部署保留已有游戏设置，斯诺克默认下架。版本、健康状态和上下架记录在发布时单独核对。
