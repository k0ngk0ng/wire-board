# 并行战斗房间的帧与存档验收

单房间已达到原版持续动画预算。两个战斗房间同时推进时，采样客户端仍会掉帧。先验证规则回调是否挡住其他房间读取已提交的快照，再测真实客户端。在证据不足时保留多房间结果为未通过。

## 实现前失败场景

1. 一个房间的规则计算未返回，另一房间的计时、WebSocket 快照或输入也停止。
2. 旧规则结果在新输入、保存、关闭、续玩或换局已提交后才返回，却覆盖了较新的权威状态。
3. 同一房间重复启动多个规则计算，积压或重排原生帧；多房间超出有界计算容量时产生无界队列。
4. 超时、规则异常、事务写入失败或服务关闭时发布半完成的状态，或用尚未提交的候选覆盖最后一份可恢复存档。
5. 输送两房间的画面时混用席位数据或发送游标；慢连接不得持有规则、保存或其他连接等待的锁。
6. 单房间、另一个房间空闲或只看逻辑帧通过时，把结果当作并行且有慢端的完整验收。记录两个真实运行版本、连接状态、服务器计时、实际像素和失败原样本。

## 修复与复测

使用真实认证的两个 HTTP 房间和快房间 WebSocket 做回归。慢房间的规则挂起时，旧实现 500 毫秒内只收到 1 个快房间游戏帧，读取随后超时。修复后同一测试在慢规则未返回前收到 5 个递增帧，慢规则只开始一次。关闭慢房间并写入 SQLite 后才释放计算，迟到结果未覆盖已关闭的内存或数据库版本。

只有已声明截止帧能力的模块使用新调度。每个房间最多计算一帧，服务内最多四个并行候选，最早截止优先。规则回调在独立候选上执行；开始计算时取当前时钟，提交时确认仍为同一权威房间对象、版本和对局。版本冲突时丢弃结果，连续两次冲突后复用原同步入口，防止高频输入长期推迟计时。关键帧仍经原事务再发布。共享平台在竞态检测下通过原帧原子性、慢连接、连接游标、定期检查点和并行隔离测试。

| 场景 | 手机实际画面变化／秒 | P95 画面间隔 | 同窗服务器计时 |
| --- | ---: | ---: | ---: |
| 修复前，两个房间推进，RTT 203 ms | 10.36 | 298.4 ms | 275 次 |
| 修复后，同样两个房间推进，RTT 201 ms | 25.09 | 50.5 ms | 274 次 |
| 修复后，两个浏览器同时在线，RTT 200 ms | 24.91 | 50.5 ms | 275 次 |

同时在线试验在同一个服务启动两间真实单人原版战斗房。手机在前台采样 5.5 秒，记录 137 次实际箭头像素变化；另一个桌面客户端在同一时间窗收到 137 个游戏包，原版箭头变化 136 次。手机游戏消息 P95 为 4106 字节。桌面并行结果为 WebSocket 收帧，未把后台标签的接收数当作桌面画布刷新率。对照测量使用相同原版规则和判定窗口。

本轮再次通过原版手机战斗的两次保存续玩：保留法术菜单和已提交治疗指令，治疗完成后另两名队员继续按原版规则行动，3 文战利金只发一次。原生 25 帧／秒和离线后剩余 17 毫秒复核也通过。一次重跑使用了上一阶段的默认文件名，已从校验归档恢复原字节，新结果移到独立路径；旧 7986 项摘要均通过。原版核心与资源未变、无上传。

以上覆盖两个房间的原版指令箭头、共享服务和定时隔离。尚未用此结果验收密集法术动画、任意房间数、断网恢复、隐藏记录剩余部分或正式发布。

## 复现

```sh
. scripts/env.sh
go test -race -v ./internal/server -run '^TestDeadlineRoomBlockedRulesDoNotStopOtherRoomOrOverwriteClose$' -count=1 -timeout=2m
PAL_CADENCE_E2E=1 PAL_CADENCE_SCOPE=mixed go test -v ./internal/server -run '^TestPALBattleCadenceRetainsOriginalFramesAndSavedDelay$' -count=1 -timeout=3m
PAL_BOARD_BROWSER=1 PAL_BOARD_ENTRY=battle PAL_SCREEN_BATTLE_CURRENT=1 PAL_CADENCE_TRACE=1 PAL_SELECTION_WEB=1 go test -v ./internal/server -run '^TestPALBoardBrowserFixture$' -count=1 -timeout=25m
```

浏览器测试服务使用当前前端产物的独立副本 `.local/pal/selection-web-dist`。先运行 `python3 -B games/pal/tests/selection-performance-browser.py battle optimized --continuous --scope mixed-live-peer --keep-open`；该程序采完单房间后写入其结果目录的 `peer-ready.json`。看到信号后，在同一测试服务运行 `python3 -B games/pal/tests/selection-performance-browser.py battle optimized --continuous --phone --delay 100 --scope mixed-live-phone`。第二个程序结束后，在第一个结果目录创建 `peer-stop`，让它保存两个房间并行期间的协议观测并关闭。旧前缀和产物在隔离副本中复跑；校验命令为 `python3 -B games/pal/tests/mixed-room-evidence.py`。
