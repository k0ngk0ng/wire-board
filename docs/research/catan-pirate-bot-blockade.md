# 海盗群岛电脑堵路问题

2026-10-11。CI 分片 9 的六人「海盗群岛＋友善强盗＋港口霸主」在 10000 步后仍未结束。本地复现后保存了完整牌局及最近 32 步；问题发生在第 333 轮，港口霸主持有者已有 14 分，但自身要塞仍有 3 层防御。

## 已确认原因

另一位已夺回要塞的电脑在外岛修路，占用了前者抵达要塞的最后一条边。其余已夺回要塞的玩家仍未达到组合所需的 11 分；14 分的玩家则无法满足夺回自身要塞的条件。因此对局持续掷骰、弃牌和结束回合，却无法完成。

冻结状态保存在 `internal/game/testdata/catan-pirate-harbor-stalled.json`，来源是本地真实 HTTP 对局，不含生产数据或服务器配置。原 E2E 现在会在失败时保存状态、阶段、行动玩家、最近操作及托管／超时标记，供 CI 上传，避免只留下「incomplete game」错误。

## 电脑策略调整

按已接通的海盗群岛规则，主岛外的新村庄位置限于本人的滩头，夺回的要塞也可作为村庄／城市；两处都经远征船线到达，且此剧本没有最长路线奖励。因此普通海盗群岛中继续修建外岛道路不能增加电脑的可建设位置或分数，却可能堵住其他远征线。

电脑在正常行动时不再为这种外岛道路花费资源。「免费道路」出牌前会检查完整的合法放置路径，实际执行时沿可完成的安全选择继续。若玩家已打出该牌再交给托管，仍保留合法动作兜底，完成卡牌要求。

这是电脑决策调整。玩家手动建设的合法性、地图、分数门槛、要塞要求及已有存档不变。城市骑士、蛮族等模块可能让道路具有其他用途，因此保留这些模块原有的道路候选，不套用普通海盗群岛的零收益判断。

## 验证

先用冻结样本还原「最后一条边尚未被堵」的决策前提，复现普通建设和免费道路两条路径，再修改电脑策略。回归检查确认电脑不再重建堵路，同时该位置对手动操作仍合法；额外模块的候选保留检查也通过。

海盗群岛三至六人、普通／Helpers 的原完整电脑局回归通过。原失败配置另有三场真实 HTTP 完整局通过，分别为 901、683、858 步，均包含主动托管、超时托管、恢复手动、存盘重启、库存和观战隐私检查。新增冻结存档回归在本机 ARM64 和 amd64 均通过，`go vet ./...` 通过。

```sh
source scripts/env.sh
go test ./internal/game -run '^TestCatanPirate(BotAvoids|RoadBuildingAvoids|RoadPolicyPreserves|IslandsBotsComplete)' -count=1
go test ./internal/server -run '^TestCatanFriendlyPublicFullHTTPGames$/^harbors=true$/^pirate_islands$/^6$' -count=1 -v
```

本轮证据：`.local/pirate-harbor-six-sample.log`、`.local/catan-harbors-failures/failure-3128726333.json`、`.local/pirate-bot-before.log`、`.local/pirate-bot-regression.log`、`.local/pirate-policy-final.log`、`.local/pirate-harbor-six-after.log`。
