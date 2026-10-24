# 乒乓球验收

## 范围

本次新增独立游戏 `tabletennis`，归入「体育」，沿用公共大厅、房间、加载、控制器、托管、音效、全屏与结算。实现前失败场景保存在 `games/tabletennis/CONTRACT.md`。检查只覆盖本游戏及其接入涉及的公共契约、编译和加载边界，不运行其他游戏的完整对局矩阵。

## 已验证内容

- 服务端规则：发球半台、发接球轮换、11 分／领先 2 分、决胜局换边、旋转与弹跳、拍位和轻度辅助、空挥后重试、非法席位／旧输入／重复输入、补偿期限及终点事件顺序。
- 24 组自然完整模块对局：单打与三种双打分队 × 两种赛制 × 三档电脑水平。仅推进测试时钟，不注入得分、球路或胜负；所有行动经过生产模块接口。
- 真实 HTTP、WebSocket 与 SQLite：自然单／双打完整比赛、真人和电脑混合、两名真人同队、主动托管与恢复手动、重复请求、并发、恢复存档、观战鉴权及含电脑不计分。
- 浏览器：PC 1440px、手机 390px／320px，三种场馆，四个双打席位，原生鼠标／触摸抽球和削球，全屏键盘操作，触屏横向跟手与取消，退出房间释放 WebGL。
- 三种尺寸均检查倒计时文字的实际边界、最终样式对比度和全屏前后 Canvas 身份。计时文字处于可视区域内，比分与状态文字对比度均超过 4.5∶1，全屏不重新挂载棋盘。
- 真正阻断 TCP 连接后，旧画面不能继续操作；恢复及刷新保留比赛，不自动托管，不重新暂停已开局的其他玩家。
- 50／150／250ms 的测试代理延迟分别通过鼠标挥拍检查，保留输入事件和逐帧显示记录。
- 150ms 延迟、双向每次转发最多 ±60ms 抖动下，原生挥拍、本地即时反馈和断线恢复通过；浏览器比赛自然打到 12∶14，覆盖平分后的领先 2 分判胜，公共结算可关闭并重新打开。
- 21 项客户端针对性检查：输入序号／取消／重试、预测边界、与服务端一致的 288 个轨迹采样、坐标投影、音效时序和恢复后的旧事件抑制。
- 四段原创音效的许可、生成器摘要、内容摘要、解码和构建副本均通过检查；OSS 对象完成不可变上传与回读核验。

手机证据来自 Chromium 窄屏与原生触摸事件模拟，不代表 iOS／Android 真机测试。电脑矩阵使用加速测试时钟；HTTP 和浏览器完整对局使用真实时钟。这两类证据分别保存。

画面示例：[PC](screenshots/tabletennis-desktop.png)、[320px 手机](screenshots/tabletennis-mobile.png)。

## 复现

先安装项目依赖，再从仓库根目录执行：

```sh
npm --prefix web run build
node --test games/tabletennis/tests/*.test.mjs
go test -count=1 -timeout=12m ./games/tabletennis/...
go test -race -count=1 -timeout=3m ./games/tabletennis/tests -run 'TestTableTennisConcurrentInputsAndRecovery|TestTableTennisHTTPServeTimeoutAndReclaim'
python3 -B scripts/check_game_boundaries.py
node scripts/game-loading/check-build.mjs
python3 -B scripts/tabletennis/verify-assets.py --dist --decode-audio
python3 -B games/tabletennis/tests/accept_browser.py
```

浏览器脚本需要 `agent-browser`、Node.js、Go 和本地 Chromium；音效检查需要 `ffmpeg`／`ffprobe`。本地测试账号、数据库和浏览器 profile 仅用于验收，脚本结束后关闭浏览器并删除 profile。

可使用 `TT_BROWSER_COURT=club|arena|heritage` 选择场馆，`TT_BROWSER_SEAT=1|2|3` 检查其他席位，`TT_BROWSER_LATENCY=50|150|250` 添加真实 TCP 延迟。`TT_BROWSER_NETWORK='{"delay":150,"jitter":60}'` 添加有界抖动；`TT_BROWSER_RECOVERY=1` 检查断线与刷新；`TT_BROWSER_MOVEMENT=1` 检查原生移拍／取消；`TT_BROWSER_FINISH=1` 通过主动托管自然打完，并关闭／重开公共结算。`TT_BROWSER_REPORT` 可指定独立的结果目录。多个浏览器任务须顺序运行，避免争用同一 fixture。

产物位于 `.local/tabletennis/`：`bots/` 为 24 组初态、行动和终局；`acceptance/<报告名>/` 为截图、输入、逐帧记录和源文件／构建摘要。真实 HTTP 对局沿用公共测试工具的 `.local/poker/tabletennis/http/` 路径。最终交付会将通过的证据归档到主工作区 `.local/tabletennis-delivery/`。
