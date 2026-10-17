# 低跳投篮动作 5A／5C

实现前失败场景：

- 5A／5C 被默认分派误报为抱篮筐动作；不能因此改成待机、普通跳跃或强迫放下。
- 原 bank 6 `$9040` 初入时按席位方向选择目标，而非按当前面向；读取本人高度、当前球架选择和装备覆盖。继续执行不得重置弹道。
- 初入球动作直接写 2，5A 另把球姿势写为 127；5C 保留原姿势。保持球主人和原请求。
- 动画、原 `$99CF` 扣篮检查、场地运动、落地后的动作 `0x16` 必须依原顺序执行。失败扣篮查询仍写入场地一侧编号。
- 两种动画帧与镜像、原计数器、JSON 恢复和原随机流必须精确保持。
- 球队 4 真实首失败（video 3611）须保持配置、种子与时钟继续。

## 证据与结果

REA `ev_a251c12a335529c38c94a5a5fc6a0f0f68b831ccba0acd7103dd62ee76821e4c` 对 `$9040` 的反编译、完整函数分析与快照保存在 `.local/basketball/rea/low-jump-shot/analysis/`。弹道共享 `$E003/$E078` 的既有恢复；本次提取成 helper，3E 原调用回归通过。

`verify_low_jump_shot.py` 保存 336 次原函数完整调用，覆盖四角色、5A／5C、六个高度边界及普通／高弧／电脑与真人上方向／装备／特殊标志。另有 8 条仅初始化一次的连续原函数链，共 368 次调用。比较完整世界状态、球主人与请求、发起者、随机状态、轨迹和 JSON 恢复，没有把配置化函数执行称为自然比赛。

实现前 `before.log` 两项均因未恢复动作失败；首次连续链捕获了动画 `FD FF` 的计数语义差异，保留 `after-overlay.log`，最终按原 `RepeatUntilCount(255)` 实现。并行开发一度产生其他尚未实现 API 的编译错误；临时 overlay 仅屏蔽两份当时无法编译的其他任务测试，并保留在证据目录。最终 `native.log`、`race.log`、`wasm.log` 均为正常包、无 overlay，全部通过，同时覆盖既有 3E 发射和扣篮查询回归。

真实球队 4 首失败状态从视频帧 3611 连续 120 帧并逐帧 JSON 重放通过。同一场地 1、球队 `[0,4]`、角色 `[0,1,0,1]`、种子 `0x12345678` 的完整赛诊断推进至视频帧 8729／逻辑更新 2907，下一缺口为角色 2 的原攻击 handler 17。该诊断源代码前后哈希一致，保存于 `.local/basketball/low-jump-team4-20261018/native/`；不能称为球队 4 整场通过。

复现定向验证：

```sh
source scripts/env.sh
.local/board-art-env/bin/python scripts/basketball/verify_low_jump_shot.py
go test ./games/basketball/engine -run '^Test(LowJumpShot|OriginalLaunchedPlayerPrepareAndComplete|WorldLaunched|OriginalDunkSearch)' -count=1 -v
go test -race ./games/basketball/engine -run '^Test(LowJumpShot|OriginalLaunchedPlayerPrepareAndComplete|WorldLaunched|OriginalDunkSearch)' -count=1 -v
GOOS=js GOARCH=wasm go test ./games/basketball/engine -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" -run '^Test(LowJumpShot|OriginalLaunchedPlayerPrepareAndComplete|WorldLaunched|OriginalDunkSearch)' -count=1 -v
```
