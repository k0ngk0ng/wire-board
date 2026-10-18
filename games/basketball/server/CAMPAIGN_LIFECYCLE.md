# 战役房间生命周期

开发模块已接入新战役、原密码导入、初次准备、赛后奖励、失败继续和下一场准备。单场结果不再直接结束整段战役；只有通关或主动结束战役才生成平台最终结果。后续已连接前端入口和准备弹窗，见 [界面验证](../web/CAMPAIGN_UI_VERIFICATION.md)；仍未注册生产游戏或发布。

## 入口和初次准备

房间选项增加可选的 `campaign`：

```json
{"mode":"solo","campaign":{"password":"020010","opponent":4,"tactic":4}}
```

省略 `campaign` 保持已有单场行为。省略原密码表示新战役；支持 `solo` 和 `cooperative`，不把双人对战当成原版合作战役。密码中的人数必须与房间一致。未指定对手时，按原地图顺序选择当前可选的第一队；主客场由进度决定，不由客户端任意指定球场。

`Configure` 返回规范化的 `campaign.preparation` 和 `availableOpponents`。准备状态保留库存顺序及四名热血球员的装备；初次准备允许的物品总数、类别和进度必须与空战役或原密码重建的库存一致，不能通过客户端选项授予物品、增加胜场或更换奖励表。

等待房间复用平台的 `Prepare`、席位身份、房主权限和房间版本。支持 `opponent`、`character`、`tactic`、`equip`、`unequip`；共享装备与对手由房主调整，其他真人只能选择自己的球员。战术修改仍需己方一名真人和一名电脑。装备属于人物槽位，换人不会把装备移到网络席位上。

实际 Start 才执行原奖励表抽取和原对手阵容选择。平台提供新战役边界的随机状态；这里不声称复刻 FC 启动 RAM 或菜单等待时间。原密码入口不重新抽奖励表。开局使用已确认的准备状态，并直接进入原比赛 Runtime。

## 赛后状态与继续

`State.campaign` 保存 `stage`、`round`、`revision`、`decisionAt`，以及可选的 `settlement`、`preparation`。阶段如下：

| 阶段 | 行为 |
| --- | --- |
| `playing` | 运行当前原版比赛，继续使用现有中场操作 |
| `reward` | 冻结已确认的胜利结果，展示原固定奖励及是否存入库存 |
| `defeat` | 冻结已确认的失败结果，允许继续或结束战役 |
| `prepare` | 调整下一场对手、己方人物、装备和队友战术 |
| `complete` | 原进度达到 28，生成合作通关结果，不再授予普通奖励 |
| `quit` | 玩家结束战役，生成无赢家的合作结果 |

结算只来自 `engine.SettleCampaignRuntime`，读取时核对结算与冻结 Runtime 一致。恢复、观战和重复请求不会插入第二份奖励。Continue 保留现有进度、库存、装备和随机状态，不重新解析原密码。

赛后操作使用：

```json
{"type":"campaign","epoch":"当前比赛标识","campaign":{"type":"continue","revision":1}}
```

仅未托管的第一席真人可从这个请求入口管理赛后共享菜单。所有修改绑定 epoch 和战役 revision；下一场 Start 只执行一次原阵容抽取，使用已有随机状态。新场次更换输入 epoch、重置该场控制器传输队列和比分，保留战役进度与装备。旧场次的控制器及准备请求不能作用于新场次。

平台托管通过同一个菜单状态转换函数确认默认选择；场内策略仍由原 AI 执行。托管菜单确认等待 3 秒，投影提供 `automaticAt`，供前端显示倒计时。玩家在菜单停留后新开启托管，会获得新的 3 秒期限；恢复既有托管存档不重置期限。取消托管和恢复存档不会修改冻结的原比赛结果。

## 验证

先列出失败场景并编写模块测试，再实现接入。新战役入口与下一场准备均保留初始失败记录。另通过先失败的真实 HTTP 测试确认了「旧菜单期限导致新开启托管立即开赛」，再修复倒计时。

- 新战役与原密码入口区分奖励表抽取；原阵容抽取和随机寄存器连接一致。
- 初次准备的真实 HTTP／SQLite 流程保留 `020010` 导入的装备、人物 2、战术 2 和对手 5，并实际初始化主场比赛。
- 从源配置生成的正常比赛得到 24:23；胜利后保留装备、领取原奖励，进入对手 4／场地 4／Period 4 的下一场。
- 同一配置的已验证中场编辑流程得到 18:26；Continue 保留编辑后的装备，并进入主场／Period 2 的下一场。
- 真实 HTTP／SQLite 赛后流程覆盖观战拒绝、奖励及下一场准备分别重启恢复、托管倒计时、下一场新 epoch、取消托管接管。
- 专项 race 检查通过；关联回归有 22 个顶层测试、38 个测试结果通过，无失败。20 个游戏目录边界检查通过。

赛后 HTTP 验收的 Start 包装器安装由实际 Go 比赛生成的结果快照，之后使用真实模块、身份、计时器和 SQLite。没有改写比分、缩短比赛或替换 AI；该测试仍是受控结果边界验收，不是从自然房间创建开始运行完整战役。模块测试明确区分新入口、实际比赛轨迹、源规则组合和平台结果封装。

产物在工作树 `.local/campaign-lifecycle/`，包括 `before.log`、`preparation-before.log`、`default-opponent-before.log`、`countdown-before.log`、`native.log`、`race.jsonl`、`regression.jsonl` 和 `countdown-after-race.log`。真实 HTTP 记录在 `.local/poker/basketball/http/TestCampaignEntryPreparationHTTP.json` 与 `TestCampaignLifecycleHTTP.json`。

复现：

```sh
source scripts/env.sh
go test ./games/basketball/server -run '^TestCampaign(RoomEntry|Entry|ResultContinue|Quit|PasswordDefault|LifecycleHTTP)' -count=1 -v
go test -race ./games/basketball/server -run '^TestCampaign(RoomEntry|Entry|ResultContinue|Quit|PasswordDefault|LifecycleHTTP)' -count=1 -v
```

## 未完成范围

后端按原战役规则允许七个对手和四名己方球员；这是未注册生产模块的开发能力，不代表所有组合已验收。原单场入口的七场地及原人物开放限制未扩大。前端已接入战役入口、图像准备、装备原名、赛后弹窗、托管倒计时和旧索引保护；装备属性预览及效果说明仍需补齐。

本次只承接已恢复的结算数据和菜单语义，未播放原奖励表演及结局动画。`complete` 的平台结果映射不能代替原结局还原。完整 14 胜战役、后期难度、全部装备和人物组合、真实双人操作、物理设备音效与素材发布仍未完成。原版悬空球相关失败继续保留，本次不修改物理或用提前胜利绕过。

规则来源见 [战役调查](../CAMPAIGN_EQUIPMENT_AUDIT.md)、[原结算证据](../CAMPAIGN_SETTLEMENT_SOURCE.md)、[新战役抽取](../CAMPAIGN_NEW_GAME_VERIFICATION.md) 与 [阵容选择](../CAMPAIGN_ROSTER_VERIFICATION.md)。
