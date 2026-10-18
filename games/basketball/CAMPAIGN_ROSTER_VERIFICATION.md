# 战役敌方角色自动选择

`SelectOriginalCampaignRoster` 恢复原战役模式0／1确认成员时的敌方选人：第一名敌人固定为角色槽0，第二名按一次原随机返回值的低2位查 `[1,2,3,1]`。输出完整四角色及更新后的共享随机状态，不计算等级、不初始化比赛，也不修改世界。

## 实现前列出的失败场景

先在 `.local/basketball/rea/campaign-opponent-roster/FAILURE_SCENARIOS.md` 记录失败清单，再编写测试、保留缺少API的红灯，最后实现组件。核心要求包括：不随机第一名、不改为均匀三选一、不重复抽取、不在NewRuntime／恢复／ACBB时重抽、不覆盖己方角色、不把Mode128／129对战选人自动化。非法参数须不推进随机状态。

## 函数、表和顺序

| 原位置 | 行为 |
| --- | --- |
| bank3 `$802A` 分派表 | 阶段6、13都指向 `$894E`；已保留的正常战役菜单均从阶段6进入 |
| `$894E..$895D` | 未进入时标记阶段已进入并清 `$06AC`，该次不抽选人随机数 |
| `$8960..$896A` | 已进入后先完成原淡出调用 `$C021` |
| `$896D` | 检查Mode符号；正常战役0／1走同一非负分支 |
| `$8974` | 待提交敌方第一个槽 `$03CD` 写0 |
| `$8977` | 原 `$C018` 恰好调用一次 |
| `$897A..$8980` | 随机返回值 `&3`，查 `$8986` 的 `01 02 03 01`，写 `$03CE` |
| `$89AC..$89C2` | 公共菜单清理并进入阶段31；不属于本次纯选人组件 |
| `$B2EF..$B2F8` | 待提交 `$03CB..$03CE` 复制到实际角色 `$03E3..$03E6` |
| `$B2FA`／`$ACBB` | 用已提交角色计算等级；这里不重新选择角色、不额外抽选人随机数 |

`$03E3..$03E6` 在菜单中也用于预览角色，不能把预览值当作已确认阵容。当前API输出的是可供原提交阶段使用的最终阵容。

调用者主循环本身也会推进共享随机数，不能把它和 `$8977` 的一次调用合并。组件接收**实际确认点的当前随机状态**。普通创建、刷新、JSON恢复或重新计算等级不能隐式调用；新的成员确认才产生新的选择。

## Mode、Period和队伍的影响

- Mode0与Mode1选择规则相同。该叶函数不读取Period、敌方Team、己方角色或装备，不按回合或对手加权。
- 第一名为角色槽0，不是平台席位0；最终放在 `Characters[2]`，由敌方Team决定人物身份。
- 表为 `[1,2,3,1]`，没有重复人物重抽。若低2位均匀，角色槽1占一半，2／3各占四分之一；实际选择由共享随机序列决定，不额外保证频率。
- 原Period在后续 `$B2B6..$B2BF` 参与场地选择（bit1置位时写Court0），不是自动选人概率。已保留Period1半场确认再次执行同一选择路径；不能以奇数节为由跳过抽取。
- 原负Mode分支不走该随机调用。它检查菜单状态 `$03D2`；负值时转Mode128并把敌方控制器 `$04EA` 设为255，非负且当前选择队伍0时进入第二队选人阶段11。这里的条件是 `$03D2`，不能误读为 `$04EA`。
- 平台组件只开放已确认的Mode0／1。原叶函数仅检查符号，可能对其他非负Mode执行相同指令；本API拒绝2～127属于开放边界，不代表原ROM主动拒绝这些值。
- 己方两个槽必须在0～3且不同。这是已确认成员选择的API前置条件；不把未选完的菜单值255传入比赛。

## 正常菜单证据与原CPU对照

全部复用已有自然菜单记录，没有重新跑整场。

| 记录 | 自动选择边界 | 后续真实提交／等级边界 | 结果 |
| --- | --- | --- | --- |
| `campaign-audit-first-match-confirm` | frame769→770，Mode0、Period0 | 1612→1613 | `[0,1,0,1]` |
| `campaign-audit-coop-first-match` | frame769→770，Mode1、Period0 | 1565→1566 | `[0,1,0,1]` |
| `campaign-audit-equipped-home-match` | frame3410→3411，Mode0、Period2 | 4194→4195 | `[0,1,0,3]` |
| `campaign-audit-loss-continue` | frame11009→11010，Mode0、Period1半场 | 11086→11087 | `[0,1,0,1]` |

020010装备主场路线在frame3410的选人前随机状态为 `B5A0CB8A`；一次原 `$C018` 后为 `5AD065C5`，返回字节 `$5A` 的低2位为2，所以第二名敌人选槽3。之后 `$B2EF` 提交角色并执行真实 `$ACBB`，20个等级字节与下一自然帧全部一致，期间随机调用次数为0。

- 392次原选人调用：Mode0／1 × 7个Period样本 × 7个敌队 × 4个随机低位。待选己方角色保持不变，敌方为 `[0, table[value&3]]`，随机只推进一次。
- 64次Mode128／129边界调用，实际覆盖 `$03D2` 的正负和当前选择队伍，证明不抽本次自动选人随机数；组件因此拒绝这两个模式，保留输入随机状态。
- 4条自然菜单边界，原指令输出与相邻已记录帧的角色待选值和完整随机状态一致。
- 4条自然提交边界，原角色提交及 `$ACBB` 后全部20个等级字节一致；不属于组件的生产职责，单独证据保存在 `.local/basketball/rea/campaign-opponent-roster/original-commit-probes.json`。

ROM、原记录及夹具SHA-256见 `engine/testdata/campaign-roster-source.json`。正常调用夹具、边界和4条自然边界合并在 `campaign-roster-calls.json.gz`。

## 统一确认清理的后续接线要求

原 `$896D` 不止选人。有效普通战役分支之后还经过 `$89AC..$89C2`：

- `$03DB = 0`：菜单预览对象槽5的Kind清零；不能误当作修改真实比赛敌方角色。
- `$06D7..$06DF` 共9字节清零：此时是菜单工作状态。在比赛阶段同地址又是场地／通道等状态，恢复时须按当前阶段解释，不把当前玩法状态随意清空。
- `$0589 = $1F`：进入未初始化的阶段31（战术等后续确认界面），尚未直接开始比赛。
- 自动选人已写的 `$03CD/$03CE` 和更新后的共享RNG保留，等待后续 `$B2EF` 提交；菜单中的 `$03CB..$03CE` 又与比赛HUD缓存地址复用，不能把它们当两份独立的持久真相。

另用4条相同自然菜单边界执行了原 `$896D→$8088`，包含上述清理；模式、待选阵容、阶段、Kind5、全部9字节工作状态和RNG均匹配相邻原帧。证据为 `.local/basketball/rea/campaign-opponent-roster/original-confirmation-cleanup.json`。本次纯选人API没有实现这些菜单状态命令；半场确认的完整事务仍待root接线，不能仅在Start分支随意插入一次随机调用就声称原路径完成。

## 实现与验证范围

API为：

```go
SelectOriginalCampaignRoster(mode uint8, teammates [2]uint8, random RandomState) (CampaignRosterSelection, error)
```

结果包含 `Characters [4]uint8` 与 `Random RandomState`。错误结果为未推进的输入随机状态和空阵容；不返回部分角色。除此之外没有副作用。

测试覆盖原调用、自然边界、负Mode边界、非法模式／重复或越界己方角色，以及全部合法己方组合。调用时机和一次确认只调用一次由上层集成验收；本次没有修改 `campaign_runtime`、settlement或平台UI，也不把独立组件通过当作完整战役已完成。

**后续实际接线：** 带战役快照的 `AwaitingNextPeriod` → Start 默认确认现已调用本选择器，并将原清理、角色提交、等级重算和四人物更新合为事务，见 [半场确认验证](CAMPAIGN_INTERMISSION_VERIFICATION.md)。本报告上方纯组件和源采样范围不变；主动修改己方角色／装备／战术的半场界面尚未接入，不能将默认确认通过写成完整菜单已恢复。

REA已解析证据为 `ev_9c7c151477c4c3be70add1a38a4bac802ebfc370cd36c27318f9a45883cbdea1`，位于 `.local/basketball/rea/campaign-opponent-roster/analysis-corrected/`。初次工具调用遇到工作树与原目录Swift模块缓存路径冲突，失败证据保留；改用已有rea.sh的规范化真实路径后完成，没有重装或修改工具配置。初次错误的872A种子位于指令参数中，没有使用其失败结果；有效默认值块从872C解析。

可重复执行：

```sh
source scripts/env.sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_campaign_roster.py
go test ./games/basketball/engine -run '^TestOriginalCampaignRoster' -count=1
go test -race ./games/basketball/engine -run '^TestOriginalCampaignRoster' -count=1
GOOS=js GOARCH=wasm go test ./games/basketball/engine -run '^TestOriginalCampaignRoster' -count=1 -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec"
```


定向验收结果：Native0.587秒、race1.698秒、Wasm0.250秒通过。红灯、通过日志与摘要均保存于 `.local/basketball/rea/campaign-opponent-roster/`。这些结果只覆盖本独立组件及已保留的原选人边界；完整赛后／半场运行验收由相应集成负责。
