# 玩家遮挡移动石头的识别验收

瀑布移动石头被已经完整识别的玩家脚部遮挡时，现在可以从剩余像素恢复位置。生产改动仅为 `bot_vision_player_support.go` 和 `bot_vision.go` 的最小接线；没有修改电脑策略、导航或 Session。

## 识别边界

- 仅适用固定视觉标签 `Class=enemy, Kind=16, Sprite=72`，不使用当前关卡筛选，不扩展为通用敌人局部匹配。
- 遮挡掩码来自已经接受的完整、精确玩家匹配，只覆盖模板不透明像素。玩家局部候选、透明孔洞、World 和 OnEnemy 均不能提供掩码。
- 所有未遮挡的不透明像素都必须精确匹配，包括暗色轮廓。遮挡不超过 25%；至少保留 75% 的整体像素和 75% 的非暗细节，至少 12 个细节点、两种非暗颜色，细节跨度达到宽度的 75% 和高度的 50%。
- 仅在公开阶段为 `battle` 时恢复，搜索限于玩家掩码附近。已有完整石头匹配不重复添加；候选重叠且锚点或含义不一致时拒绝识别。

`applyBotVisionMarkers` 在恢复前执行，确保介绍、菜单、过场和终局不会误用战斗阶段。地形识别原有路径保持不变。

## 真实画面与重放

正例是原诊断输入重放得到的 **pre-step 第 25926 帧**：完整玩家位于 `(158,99)`，移动石头位于 `(170,127)`。测试素材为 `games/contra/engine/testdata/bot-waterfall-player-covered-stone.png`，SHA-256 为 `c91072f742bd1ad8004725666ce01f7638ca6aa109dd6032f41898285add2355`。

原 `final.png` 没有完整玩家匹配，只产生局部候选。因此保留为负例 `bot-waterfall-partial-player-stone.png`，不能用它给石头提供遮挡掩码。

| pre-step 帧 | 完整玩家 | 修复前石头 | 修复后石头 |
| --- | --- | --- | --- |
| 25924 | 无 | `(168,127)` | `(168,127)`，保留已有完整匹配 |
| 25925 | 有 | 无 | `(169,127)` |
| 25926 | 有 | 无 | `(170,127)` |
| 25927 | 无 | 无 | 无 |
| 25928 | 有 | 无 | `(171,127)` |

`scripts/contra/replay_player_support.go` 从新 Session 重放记录的合法按键，只应用原记录明确声明的 21 次补命，并校验补命前值。重放不调用电脑策略。五张 PNG 和完整 Session 在修复前后逐字节一致，只有视觉观察中的石头结果改变。该记录用于已授权的补命诊断，验收结论限于视觉识别及重放一致性。

完整 Session SHA-256 为 `12521de4c04d69694e9404f18592d5ae50a8ceb0ff1642056f201b8ef4e410b6`。原始按键与补命记录保存在 `games/contra/engine/testdata/player-support-replay/`。最终输出保留于 `.local/contra-player-support/recheck-final/`；修复前观察另存为 [before-observations](evidence/bot-player-covered-supports-before-observations.json)。

## 测试结果与复现

失败场景清单与测试先于实现。真实 PNG 和合成有效遮挡用例的失败记录保留在 [real-red](evidence/bot-player-covered-supports-real-red.jsonl)。测试还覆盖没有完整玩家、透明孔洞、未遮挡像素冲突、遮挡过多、细节不足、单色、暗色轮廓冲突、通用敌人、歧义锚点、已有精确匹配及非战斗阶段。

广泛视觉 race 回归通过 **33 项顶层测试、71 个测试及子测试记录，零失败**，见 [race 日志](evidence/bot-player-covered-supports-race.jsonl)。随后绑定源码的聚焦 race 通过 **3 项顶层测试、22 个测试及子测试记录，零失败**，见 [绑定日志](evidence/bot-player-covered-supports-bound-race.jsonl)。运行期间源码未变化，收尾时所有绑定文件仍匹配。

```sh
source scripts/env.sh
go test -race ./games/contra/engine -run '^(TestBotPlayerCovered|TestBotMaskedTerrain|TestBotEnergyFloor|TestBotVision|TestBuiltinBotVision)' -count=1 -json
python3 -B scripts/contra/verify_player_support.py --replay --label verification-01
```

重放要求使用尚不存在的 `label`，防止覆盖已有输出。最终校验及源文件摘要见 [verification](evidence/bot-player-covered-supports-verification.json)；生产文件、脚本、素材、测试日志、前后帧和清理清单见 [closeout](evidence/bot-player-covered-supports-closeout.json)。

清理仅删除本任务的 `replay-before/`、`replay-after/` 两份重复重放目录和三份冗余早期日志，共 17 个文件。已保留唯一最终重放、修复前观察、真实失败记录和绑定验收证据。原诊断目录、共享 Go 缓存、其他游戏、worktree 和用户素材均未改动。
