# main 实时架构接入

本轮将篮球分支 rebase 到 `44fbd7b5`，沿用平台的实时传输、状态保存和控制权管理。此文件记录接入方式及验收边界，发布结果另以对应版本的 tag 和 Actions 记录为准。

## 传输与恢复

- 正式对局通过平台 `BoardProps.act` 使用 `/api/ws?stream=1`，发送输入并接收确认和游戏快照。篮球不另建网络连接。
- 发送器读取平台 `inputWindow`：有序 WebSocket 最多保留 8 条普通未确认请求，HTTP 回退保留 1 条；取消输入另有一个有界通道。后续输入保留未确认的边沿，服务器按序号去重。
- 按键释放不必等待上一条确认。丢包、超时、失焦、身份变化或托管切换时清理旧控制；客户端不决定比分、碰撞或胜负。
- 实时游戏不因空闲自动托管。电脑和玩家主动托管仍使用恢复的原版 AI；断线按键过期只清理控制，不改成电脑。
- `CheckpointKey` 标识阶段、上下半场、战役场次、准备版本及结局确认期限。普通输入和帧沿用平台内存推进与周期保存，关键边界先保存再发布。
- 恢复保留比赛身份、进度、随机数和系统期限，清理旧物理按键。平台 `serverInstance` 更换会重建棋盘，允许恢复较低版本的检查点并拒绝旧进程输入。

## 赛后流程

比分确认与整个流程结束分别使用 `Outcome.Confirmed` 和 `RuntimeFinished`。新战役获胜后继续播放原版奖励或结局，完整保存画面、输入和声音状态；奖励完成后直接进入下一场准备，使用最终原版随机数，不重复发奖。

结局播放完毕后，真人由第一席按 Start 确认；托管席位公布 3 秒期限后确认。期限属于持久化状态，页面显示倒计时，恢复不重置期限。旧存档没有 presentation 状态时保持原有确认流程。

## 本轮验证

- `games/basketball/tests/network-input.test.mjs`：短按、连发、优先取消、乱序返回、身份更换、超时、流式窗口和 HTTP 回退。
- `TestBasketballRealtimeStreamAndRestore`：真实登录和 WebSocket 连续 8 次输入、重复 nonce、重启、旧进程和观战权限。结果在 `.local/poker/basketball/http/`。
- `TestBasketballBrowserPlatformRooms`：实际构建页面、原生键盘、PC 与 320px、准备图片和权限。测试增加 50／150／300 ms 传输延迟；300 ms 下验证松键早于首条 ACK，保留 `stream-trace.json` 和截图。
- `TestCampaignPresentationHostLifecycle`、`TestCampaignEndingConfirmationDeadline`：奖励播放、恢复、唯一结算、最终随机数、真人 Start、托管期限及关键保存边界。
- 正常时钟完整比赛使用 `TestBasketballHTTPCompletePreparedMatch` 验证房间、输入、托管、恢复和完整比赛。HTTP 是该验收驱动方式；正式页面优先使用 WebSocket。
- 全部原版选择范围及新增扣篮、传球分支的证据见 [选择范围验收](SELECTION_ADMISSION_VERIFICATION.md)。显示验收见 [赛后显示验收](RUNTIME_PRESENTATION_DISPLAY_VERIFICATION.md)。逐像素参考图、受控结局及自然完整比赛的证明范围分别记录，不互相替代。

`server/testdata/ending-needs-confirm.json.gz` 来源为原版结果确认边界与独立 Go 结局流程，不是人为制造自然十四胜记录。生成方式：设置 `BASKETBALL_PRESENTATION_ARTIFACT_DIR` 后执行引擎 `TestRuntimePresentationDisplayLifecycle`，将生成的 `ending-needs-confirm.json` 用 `gzip.compress(..., mtime=0)` 压缩。

## 复跑

先执行 `source scripts/env.sh`，缓存和临时文件留在仓库内。

```sh
node --experimental-strip-types --test games/basketball/tests/network-input.test.mjs
go test -race ./games/basketball/server -run '^(TestBasketballRealtime.*|TestCampaignPresentationHostLifecycle|TestCampaignEndingConfirmationDeadline|TestBasketballHTTPCompleteOriginalSelection)$' -count=1
BASKETBALL_BROWSER_E2E=1 BASKETBALL_BROWSER_RTT_MS=300 go test ./games/basketball/server -run '^TestBasketballBrowserPlatformRooms$' -count=1 -timeout=4m
python3 -B scripts/ci/scope.py run --game basketball --backend
```

main 仅在篮球后端受影响时运行篮球完整对局和选择矩阵；前端修改不启动这些长时验证。公共检查不运行其他游戏规则。版本 tag 复用同一 SHA 的已验证构建，不重新执行游戏验收。
