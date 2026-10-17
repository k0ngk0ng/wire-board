# 全电脑完整视频比赛验收

## 实现前失败场景与固定范围

- 新增独立TestRuntimeAllComputerCompleteMatchAndRestore，默认纽约球队与角色、seed0x12345678、四席ControllerAssignments=-1；原混合首节和完整比赛测试的配置与目标保留。
- 复用完整视频helper并显式传配置。保留48000视频帧上限、两次独立初始化和每300帧／节间的JSON恢复后下一tick一致性检查，不缩短时钟或减少恢复覆盖。
- 比较完整两节、跨节保留的实际分数、确认结果与原显示，不把6000逻辑更新的成功代替视频比赛结束。
- 四席游戏操作由原电脑AI决定；唯一外部按键仍是helper在原节间等待结束后按Start确认下一节。每个Playing状态核对实际Controls，防止配置未生效。
- 首个未恢复分支保留video/logic位置和完整前态。仅恢复原AI；若落在物理、碰撞、辅助对象等范围则交主线程，不换种子或跳错误。
- 原函数隔离验证、自然原运行对照和这项cold视频集成测试分别统计，不将其合称原版整局一致性证明。

## 首差与恢复

首次独立测试在video557／logic183触及原追逐索引88。`PoseSource`由对应动作恢复工作按056A实际写入维护；AI仅惰性读取动作源、F4深度和DA／CD坐标。具体来源、缺字段语义和原函数对照见AI_PURSUIT_ALIAS_VERIFICATION，不以Holder或LastTouch代替源字节。

首差完整前态归档为`.local/basketball/video-runtime-all-computer/first-alias88-failure.json`，来源为PoseSource恢复期间的真实失败日志并附SHA。当前成功报告更新后仍保留此失败证据。

## 完成结果

原生和Wasm各独立初始化并完整运行两次，四次均为：

- 22512视频帧、7478逻辑更新、85次JSON恢复后下一tick校验。
- 完整两节、节间原Start确认和已确认原结果显示，最终比分[45,49]。
- 轨迹SHA-256：`f9e8c5c3c7015fa98ea3cca54f23dacf052c70209ac17f6287e9f70779cfca93`。

仍保持48000帧上限和每300帧恢复检查，没有缩短时钟、固定胜负或注入快照。`Playing.Context.Actions.Controls`每次均核对四席实际电脑标志。原两个混合测试只把原默认配置显式传给共用helper，seed和配置未变；race回归仍通过原混合首节及完整比赛（22383视频／7435逻辑、比分[12,45]），同时通过全电脑完整比赛和alias88边界测试。

独立结果目录为`.local/basketball/video-runtime-all-computer/`：`native-all-computer-full-match-result.json`、`wasm-all-computer-full-match-result.json`及相邻JSONL；完整race日志为`regression-race.jsonl`。原默认报告目录与测试选择保持不变，只有显式`--all-computer`选择本项完整比赛和独立输出。

原PoseSource字段迁移后，1998/57两份原loop通过CLI与Wasm，旧字段保持不变。此项视频测试是固定冷配置的运行、时钟、显示、结算和恢复集成验收，不是原ROM全比赛逐帧oracle，也不证明战役／菜单或全部可能配置均已恢复。

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_video_runtime.py --all-computer
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_video_runtime.py --all-computer --wasm
source scripts/env.sh
go test -race ./games/basketball/engine -run 'TestPursuitAlias88|TestRuntimeColdFirstPeriodAndRestore|TestRuntimeColdCompleteMatchAndRestore|TestRuntimeAllComputerCompleteMatchAndRestore' -count=1
```
