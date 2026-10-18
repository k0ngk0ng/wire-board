# 战役赛后结算与下一场接线

`SettleCampaignRuntime` 从已经确认的真实比赛结果生成下一份战役进度。它不接受外部指定的赢家、分数或奖励，不修改原 Runtime，不推进随机数。重复调用或恢复同一份结果会得到相同的完整进度，不会向当前库存再次追加奖励。

## 实现边界

- 普通胜利：使用结果阶段已经更新的 Period／WinBits，按原奖励表取物品，插入第一个空库存格，保留顺序、重复物品和已装备物品。奖励不会自动装备。
- 满库存：与原循环一致，不越界、不覆盖已有物品；返回原展示物品，同时 `RewardStored=false`。此容量域的源探针是受控异常输入，不声称正常战役会自然获得 16 个库存物品。
- 败局：返回 `CampaignDefeat`，保持原结果回退后的进度、库存和装备；不自动选择继续。
- Period28：返回 `CampaignComplete`，跳过普通奖励。结局动画仍未接入。
- 输出保留当前原 RNG。实际确认下一场阵容时，调用 `SelectOriginalCampaignRoster` 消耗原规定的一次随机调用，再把更新后的 RNG 传入 `NewCampaignRuntime`。构造和恢复不重抽阵容。

本接口只处理数据结算，不执行原奖励展示的动画／音效／等待，也不代替服务器的进度持久化、身份验证和界面交互。不能把一次接口返回称为已完成全部赛后场景。

## 测试先于实现

先列出提前发奖、重复发奖、进度回退错误、空槽越界、自动装备、重新抽随机数和篡改结果等失败场景，再写集成和原夹具测试。缺少 API 的编译失败保留在工作树 `.local/campaign-settlement/before.log`、`before-complete.log`。新增敌方选择器接线前的失败保留在 `before-roster-handoff.log`。

720 组源结算数据全部匹配：普通胜利 560、败局 128、最终胜利 32，其中 112 个满库存案例保留原展示但不入库。测试用原指令产生的已确认结果构造隔离 Runtime 外壳，只比较结算；不把该外壳称为完整自然对局。原代码省略的显示／等待入口、实际写入和 RNG 调用均列在 [源报告](CAMPAIGN_SETTLEMENT_SOURCE.md)。

实际运行的路径为：`020010`→装备拥有的 item1→主场对阵纽约→正常完整比赛→确认结果→领取 item0→原地图剩余顺序选择 opponent4→原敌方选人→下一场 Court4／Period4。

该路径在 Native 和 Wasm 各重复两次，race 也通过。每次第一场在第 22,385 视频帧结束，比分 18:12；结算后 Period4、WinBits `[1,1]`，原装备保留，库存增加 item0；下一场第 114 视频帧进入实际 Playing，比分清零、难度为 Period4。验证重复结算不写回旧 Runtime、JSON 恢复结果相同、已击败对手不能再次选择，并保留原共享 RNG 顺序。第二场仅验证开场交接，没有宣称第二场已完整结束。

独立选人组件的 392 次原调用、64 次模式边界和 4 条自然菜单边界见 [选人验证](CAMPAIGN_ROSTER_VERIFICATION.md)。新结算和选人定向检查在 Native／race／Wasm 通过；篮球全部包编译与模块边界检查通过。未重跑全游戏规则矩阵，也未将上一轮保留的两个战役托管卡局算作通过。

## 重复验证

```sh
source scripts/env.sh
.local/board-art-env/bin/python -B scripts/basketball/verify_campaign_settlement.py --check
.local/board-art-env/bin/python -B scripts/basketball/verify_campaign_roster.py
go test ./games/basketball/engine -run '^(TestCampaignSettlement|TestOriginalCampaignRoster)' -count=1 -v
go test -race ./games/basketball/engine -run '^(TestCampaignSettlement|TestOriginalCampaignRoster)' -count=1
GOOS=js GOARCH=wasm go test ./games/basketball/engine -run '^(TestCampaignSettlement|TestOriginalCampaignRoster)' -count=1 -v -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec"
```

日志和结果索引位于工作树 `.local/campaign-settlement/`。夹具元数据保留原 ROM、基础 RAM 和生成内容摘要。

## 仍需完成

原 Mode0／1 半场成员确认也会再次选择敌方角色，当前 `AwaitingNextPeriod`→Start 仍沿用旧阵容。必须连同原确认清理、角色提交和等级重算一起恢复，不能只增加一次随机调用。服务器的赛后准备／奖励领取／继续或退出、统一平台界面、完整 14 场战役和结局仍未接通。悬空球兼容策略仍等待用户选择，不影响继续推进这些独立工作。
