# 战役准备与比赛运行时接线

状态：进行中。`NewCampaignRuntime` 已将已验证的战役准备接入真实比赛运行时，正常单人路线完成 Native／Wasm 整场验证；单人托管和合作托管仍因悬空球卡局而失败。失败测试保留，当前不能作为可发布的完整战役。

## 实现范围

- 输入为 `campaign.Progress` 和已提交的对手、四人阵容、战术、当前 RNG。主客场、Mode、热血队、装备、Period、WinBits 从准备进度派生；对手自动选人算法尚未接通，不能把显式提交阵容当作原自动选人已经恢复。
- `RuntimeConfig.Campaign` 保存最近一次确认的准备快照，包括库存和奖励表编号，胜场进度保持本场开始时的偶数基线。后续新增的半场编辑另存在 `RuntimeState.CampaignPreparation`，Start 时才原子替换当前配置。构造及逐帧复制会复制快照；JSON 恢复不重新解码密码。比赛中的半场和胜利掩码由已有阶段状态推进，未将奇数半场写回准备进度。
- 装备按人物归属传入原开局、属性、投篮、跳跃与 AI 接口。原追逐分支读取的库存槽 7 改为准备快照中的实际值。恢复校验增加每席投篮装备、库存读取值和战役进度一致性检查。
- 所有准备校验仍是结构／规则校验。调用方负责保存来源与权限，不得接收玩家自行构造的库存作为已获得物品。
- 本轮未接入服务器创建／准备界面，未接通赛后奖励确认、败局继续、下一场选择和最终结局；完整战役仍未完成。

后续引擎接线已补入赛后数据结算及下一场开局，见 [赛后结算与下一场](CAMPAIGN_SETTLEMENT_INTEGRATION.md)。服务器和界面尚未消费这些接口，半场重新选人、败局交互及完整结局仍未完成；下方首次接线的验收范围保留。

## 原版路线与验收

原正常菜单输入 `020010` 恢复 Period2、WinBits `[1,0]`、库存 item1；随后装备角色 0 的衫并进入主场比赛。原录像 `campaign-audit-equipped-home-match` 的第 4,195 帧进入场地，第 4,345 帧进入比赛：Court0、Teams `[0,1]`、Characters `[0,1,0,3]`、战术 `[4,4]`。原进入场地时的 RNG 为 `0x26DC9128`，只作为源夹具输入，不写入产品默认值。

夹具 `engine/testdata/campaign-home-entry.json` 保留配置、属性、控制器和源 RAM 摘要。`verify_campaign_home_entry.py` 从保留的原录像重新核对。测试先于实现编写，未实现 API 时的失败编译日志保留在工作树 `.local/campaign-runtime/before-complete.log`。

实际调用密码解码、拥有物品装备、战役构造器、逐帧比赛、中场确认和 JSON 保存恢复后，正常单人路线在 Native／Wasm 各连续完成两场：22,385 视频帧、7,433 逻辑帧、84 次恢复检查，比分 18:12。四次轨迹 SHA-256 均为 `aa0edb313abaf074db8b95fa815abfc506891e716d2d4d7af62ed54745346ed3`，Period2→3→胜后4，WinBits `[1,0]`→`[1,1]`。

这是从准备流程到整场的 Go/Wasm 验证，源对照范围为配置、属性、规则组件和明确原调用；不代表整场逐帧与自然菜单录像一致。装备 1 不改变这些基础属性，其释放效果另有 [原调用验证](CAMPAIGN_SHOT_EQUIPMENT_VERIFICATION.md)：3,184 次人物释放和 96 次完整动作阶段在 Native／race／Wasm 全部匹配。

## 保留的失败

在同一准备配置开场使用平台托管接口将四人交给原 AI 后，Mode0 和 Mode1 均停在第一节：球在 32 像素高台边缘变为静止悬空，时钟归零、比分 0:3，48,000 帧内未结束。两个测试仍然失败，没有换种子、改计时或跳过断言。

工作树 `.local/campaign-runtime/first-stall/` 保存第 3,420 帧确认持续静止时的前 50 次逻辑更新。第 3,345／3,378／3,381／3,384／3,387 帧的独立原球动作调用与 Go 均一致，记录在 `isolated-ball-dispatches.json`；这只能证明相同前态的球动作，尚不能排除更早的整机状态差异。

另一项 Mode128／Court1 的扩展阵容 case03 已完成原整机分段核验：分别在平台两节开场边界对齐一次，第一节 3,604 个、第二节 4,864 个完整世界状态均一致。原程序第二节也在滚离高台后进入 64 像素悬空，随后持续 2,529 次世界更新，最终时钟为 0、比分 35:26。参考运行与带观察钩子的运行最终 RAM 和每段 5 张截图均相同，另有 50 次独立球调用和 50 次连续球调用一致。证据索引为共享 `.local/basketball/rea/roster-mode128-after-corner-case03/summary.json` 和 `INDEX.md`。

这两段不能称为原菜单不中断的完整比赛，也不能扩展为全部运行时字段一致：世界比较排除投影和临时接触候选；额外 MatchState 对照仅发现 `Clock.VideoRemainder` 在原 D355 与 Go StepVideo 边界不同，最早第 10 次为 227／226，精确 CPU／NMI 采样原因未判定。其他已审计的时钟、半场、比分字段一致。Court0 的五次局部球调用不因此升级为完整整机证明。是否在复刻版修正这种原程序也会出现的卡局已询问用户，尚未收到选择，未修改原物理行为。

篮球全部包回归的结果为 588 个顶层测试通过、7 个显式可选测试跳过、1 个顶层测试失败。失败项为 `TestCampaignPasswordEquipmentRuntimeFlow` 的上述两个托管场景；服务器、音频状态和战役 leaf 包通过。整体结果仍是 FAIL。模块边界检查通过，未运行其他游戏规则矩阵。

## 重复验证

在独立工作树运行，完整测试命令会保留并报告已知托管失败：

```sh
source scripts/env.sh
python3 -B scripts/basketball/verify_campaign_home_entry.py
go test ./games/basketball/engine -run '^TestCampaign' -count=1 -v
go test -race ./games/basketball/engine -run '^TestCampaignRuntime' -count=1
GOOS=js GOARCH=wasm go test ./games/basketball/engine -run '^TestCampaignPasswordEquipmentRuntimeFlow/solo$' -count=1 -v -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec"
go test ./games/basketball/... -count=1 -json
```

本轮日志与结果索引位于工作树 `.local/campaign-runtime/`；扩展阵容复验另保存在工作树 `.local/roster-after-corner-mode128/`，仍为 12／16 组通过，失败配置 03／06／12／14 未绕过。
