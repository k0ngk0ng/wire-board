# 实时状态保存验收

普通输入和物理帧已改为内存推进，不逐条更新 SQLite 房间快照，也不逐条追加动作日志。每秒保存有变化的房间；开局、结算、托管、关闭及游戏声明的关键阶段先事务保存，再发布结果。此报告只覆盖已完成的持久化与传输阶段；总体进度见 [PROGRESS.md](PROGRESS.md)。

## 实测

使用同一个平台计数模块隔离宿主开销：4 个房间，每房间 8 名真实登录用户，每人每秒 5 次输入，输入窗口 2 秒。该样本衡量平台开销，不代表实际游戏的最大容量；延迟数据不是互联网端到端延迟。

| 指标 | 合并写库前的流式基线 | 内存与检查点 |
| --- | ---: | ---: |
| 输入／成功确认 | 320／320 | 320／320 |
| 房间快照 UPDATE | 400 | 8 |
| 新增动作日志 | 400 | 0 |
| 编码后写入快照字节 | 539,453 | 10,782 |
| 输入确认 p50 | 11.379 ms | 5.388 ms |
| 输入确认 p95 | 18.524 ms | 10.281 ms |
| 错误／未完成输入 | 0／0 | 0／0 |
| 完整状态 GET | 0 | 0 |

快照写入次数减少 98%。新实例标识等协议字段略增了单条网络消息大小；此次优化没有用遗漏确认或停止系统帧换取写库减少。80 次系统帧正常执行，帧到期延迟 p95 为 1 ms。原始数据位于 `.local/realtime-efficiency/pre-batch/load-stream-4.json` 和 `.local/realtime-efficiency/load-stream-4.json`。

## 恢复语义

- `durable: false` 表示服务器已处理输入，尚不能视为落盘。正常重连读取最新内存状态。
- 检查点保存失败不丢失待保存标记；后续保存最新状态。日志包含最后成功版本和时间。正常关闭尝试保存剩余变化，并返回、记录保存错误。
- 存储健康时，异常进程退出可能回退约一个保存周期；存储持续失败时按最后成功检查点恢复，不能保证一秒窗口。
- 真实子进程在成功确认一条未持久化输入后被 SIGKILL，重新打开 SQLite 恢复最后检查点。真人旧方向／按住开火被清理，比赛身份、进度和系统期限保持；电脑及手动托管不取消。
- `serverInstance` 区分进程实例，客户端接受新实例的较低检查点版本，拒绝旧实例帧；服务器拒绝带旧实例标识的输入。
- 结算失败不先发布胜负；缺少房间行时拒绝保存，不能留下孤立战绩。普通回合制事务语义不变。

## 验收产物

主要结果位于 `.local/realtime-efficiency/`：

- `memory-checkpoint.json`：两个有效输入、一个重复 nonce、五个系统帧，逐步写库 0 次；检查点一次保存最新状态，无变化时不重复保存。
- `checkpoint-retry-control.json`：写失败后继续保留内存状态，重试保存最新版本；托管切换立即持久化，随后不重复检查点。
- `sigkill-checkpoint.json`、`instance-restore-control.json`：进程崩溃恢复与旧实例控制拒绝。
- `critical-result-missing-row.json`：缺失持久房间记录时不发布结算，也不归档孤立战绩。
- `checkpoint-load.txt`、`load-stream-4.json`：上述独立运行的负载样本，附 CPU／mutex profile。
- `browser-checkpoint-rtt50/`、`browser-checkpoint-rtt150/`、`browser-checkpoint-rtt300-final/`：五款游戏实际键盘输入、直接状态推送、手动托管／接管、桌面与 320px 截图；300 ms 下连续运动松手和方块后续批次可早于前一条 ACK 发送。
- `bubble-eight-human-stream.txt`：泡泡堂八名真人同时放泡泡、第二标签页重复 nonce、重连保留序号、观众输入拒绝。完整状态记录另存 `.local/poker/bubbleparty/http/TestBubblepartyEightHumanStreamTabsAndReconnect.json`。
- `.local/ci/shared/result-0.json` 与 JSON 测试日志：公共平台、模块契约、HTTP／持久化和竞态验收通过。
- `checkpoint-front.txt`：五款游戏与传输辅助测试 81 项；`checkpoint-web-shared.txt`：公共前端 79 项；`checkpoint-build.txt`：前端构建。
- `checkpoint-games-race-passed.txt`、`checkpoint-map-matrices.txt`：五款游戏中已完成的包级竞态检查及泡泡堂／坦克大战完整地图矩阵；赛车单列如下。
- `checkpoint-racing-full-final.txt`、`checkpoint-racing-focused-race-final.txt`：赛车完整常规验收、HTTP／恢复／托管竞态验收。

最初把赛车完整 45 个赛道／人数／圈数组合同时放入默认 10 分钟竞态任务，运行超时。已按仓库策略拆分完整矩阵和重点竞态验收，两项最终均通过。两处旧断言要求空闲托管、重启保留油门，已按新要求改为不自动托管和清除旧控制，同时保留比赛状态检查。超时与旧断言失败不计为通过。

## 复跑

先在仓库根目录执行 `. scripts/env.sh`，所有缓存、临时文件保留在仓库内。

```sh
go test -race ./internal/server -run '^(TestModuleRealtime|TestRealtime)' -count=1
WIRE_BOARD_REALTIME_LOAD=stream WIRE_BOARD_REALTIME_ROOMS=4 WIRE_BOARD_REALTIME_PROFILE=1 go test ./internal/server -run '^TestRealtimeLoadEvidence$' -count=1
python3 -B scripts/ci/scope.py run --game shared
python3 scripts/realtime/check-browser.py --rtt-ms 300 --idle-seconds 65
go test -race ./games/bubbleparty/tests -run '^TestBubblepartyEightHumanStream' -count=1
go test ./games/racing/tests -timeout=15m -count=1
go test -race ./games/racing/tests -run '^(TestRacingHTTP.*|TestRacingRestoreReclaimAndPacketLoss|TestRacingTimerReplayAndIdleDraw)$' -timeout=5m -count=1
```

负载测量须独立运行，避免与规则矩阵或浏览器任务争用 CPU。浏览器脚本结束后删除本次 profile、socket、测试数据库和临时目录，保留截图、网络轨迹和报告。当前阶段未创建 tag、Release，也未更新生产；原有疯狂坦克工作树改动保留。

源码 SHA-256、最新结果和清理清单保存在 `.local/realtime-efficiency/checkpoint-manifest.json`。清理仅涉及本次检查点阶段已被成功验收替代的失败日志、旧 300 ms 浏览器运行目录，以及赛车旧自动托管预期的失败记录；成功报告、截图、负载基线、CPU／mutex profile 均保留。没有删除用户素材或其他游戏工作文件。
