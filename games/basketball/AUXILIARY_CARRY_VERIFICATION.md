# 辅助道具拾取与 A 投掷恢复

已恢复独立人物／道具组件，并接入 `world_actions.go` 与 `world_loop.go`。本次保留现有 Kind14 碰撞形状和瞬时物理控制器输入，不改变接触规则或完整比赛门禁。

## 原阶段

REA 证据 `ev_3939685c9d4209552369ace82e9f9ba03708e1f7fe2eb66c03a731bc74c42a7e` 位于 `.local/basketball/rea/auxiliary-carry/analysis/`。

| 对象／动作 | 原函数 | 行为 |
| --- | --- | --- |
| 人物 `$4E` | `$91A4` | 真人按接触 bit5 选对象 4／5；电脑按既有 owner 选择。设道具动作 1、owner 和附着关系，人物动画结束进入 `$20` |
| 人物 `$20` | `$87AD` | 地面持物待机；保留原待机姿态和停止运动语义 |
| 人物 `$27` | `$927F` | 向已持有道具写动作 14；人物投掷动画结束返回待机 |
| 道具 1 | `$B05D` | 初入清平面速度并标记腾空；每次通过 `$EC7B` 跟随 owner，再执行 `$EEFF` |
| 道具 14 | `$B42B` | 初入全停、保持 owner、跟随一次，播放准备动画；动画结束才转动作 3 |
| 道具 3 | `$B0B1` | 初入先读取旧 owner 朝向，再清 owner／附着；X 速度 ±12、高度速度 −3、重力 0。横速停下时改重力 1 并重置游标，落地转动作 0 |
| 道具 6 | `$B164` | 清 owner／附着并隐藏，按 `SpecialHolder` 跟随；是独立状态，不是 A 投掷链的必经步骤 |

`$EC7B` 和附着 `$EEFF` 是两个不同阶段。前者按 owner 动作／姿态写偏移及共享 `PoseSource`；后者按支撑对象和姿态覆盖整数位置，保留道具自身小数。不能以单次「贴到人物上方」代替。

## 接线契约

- `StepAuxiliaryCarrier(player, context, profile, court)` 支持 `$4E/$20/$27`，并先处理人物请求表。上下文包含 `Actor`、`Computer`、原 `$0634+actor` 的 `PickupFlags`／`PickupFlagsKnown`、两个 `Owners` 及人物 `Attachment`。返回人物状态及 `ChangeAuxiliary/Object/Action/BindOwner` 命令。真人拾取初入必须 `PickupFlagsKnown=true`；已知零值与未提供来源不同，缺来源直接报错。绑定时写 owner=actor、attachment=actor|128；对象动作在其后续调度阶段执行。
- `ResolveAuxiliaryRequest` 恢复对象 4／5 的 `$E18B` 接受表。接受的请求在选 handler 前生效；被拒绝的请求继续 pending，已处理请求不重放。
- `StepAuxiliaryCarryThrow(state, context, court)` 支持道具 1／3／6／14，并使用上述请求表。`AuxiliaryContext.Owner` 是当前原 owner，`OwnerKind` 用于附着高度；`SpecialOwner` 对应 `SpecialHolder`。结果的 `ChangePoseSource/PoseSource` 写共享 `$056A`，不是改变当前 owner。
- 传入已恢复的场地 motion binding 时，组件执行原物理阶段一次，不需要调用方重复积分。附着路径按原 `$EEFF` 直接跟随支撑对象，不执行自由运动。

`AuxiliaryState` 的 owner、attachment、动作与游标仍是唯一权威状态。本次没有新持久副本，也没有从动作反推或清空瞬时物理控制器输入。

## 验证

实现前失败场景见 `VERIFICATION.md`。两条已建立的显式 Court3 拾取／A 投掷链各执行 23 个周期，新增 **92 个**完整人物／道具动作边界。Go 保持人物和道具运动／动画连续，仅接收原输入阶段的请求、方向与接触标志；两个链的未分配控制器 3 分别保持 0 和 `0x41`。这不是完整同时比赛，也不是自然菜单／自然对局证明。

另有 **120 次**原完整道具 handler 隔离执行，覆盖动作 1／3／6／14、初入／持续、附着／自由、owner 姿态和朝向。**512 次**原 `$E18B` 指令执行覆盖 16 个动作与 16 个请求的进入位组合。来源与 SHA 保存在对应 `*-source.json` 中。

探测暴露旧人物形状的纽约物理适配会把道具动作 14 错当人物动作 14，提前变为 Falling 15。新增组件按原对象公共腾空分支处理，不执行人物专属转换；120 条原指令夹具中的第 88 条保留该具体失败输入，最终通过日志确认已经匹配原动作 14。

首轮尚未恢复的道具动作 2、附着人物 `$27` 已由下述独立扩展补齐；未知跟随源与未列出的动作仍明确报错。此次中场平地链不证明其他球场所有地形和机关已完成。

## 复跑

```bash
source scripts/env.sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/capture_auxiliary_carry_chain.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/probe_auxiliary_carry.py
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/extract_auxiliary_follow_tables.py
gofmt -w games/basketball/engine/auxiliary_follow_tables.go
go test -race ./games/basketball/engine -run '^(TestOriginalAuxiliary|TestAuxiliary|TestOriginalKind14PickupThrowContactChain|TestWorldActionsEntireCapturedCorpus|TestWorldPreparationEveryOriginalCompleteStage)'
basketball_goroot=$(go env GOROOT)
GOOS=js GOARCH=wasm go test -exec "$basketball_goroot/lib/wasm/go_js_wasm_exec" ./games/basketball/engine -run '^(TestOriginalAuxiliary|TestAuxiliary|TestOriginalKind14PickupThrowContactChain|TestWorldActionsEntireCapturedCorpus|TestWorldPreparationEveryOriginalCompleteStage)'
```

结果日志：`.local/basketball/engine/auxiliary-carry-{before,after,race,wasm}.log`，请求表先失败日志为 `auxiliary-request-before.log`。

世界接线从 `WorldInputs` 已提交修改后的 `Preparation.Contacts.Marks[:4]` 获取拾取标志。当前 `WorldLoop` 已把输入阶段的 `ChangePickupFlags` 写回该位置。通过 `WorldActionContext.PickupFlags *[4]uint8` 可空指针传入动作上下文，只有确实提供原来源时设置 `PickupFlagsKnown`；准备场景等没有该来源的路径不能默认填 0。

## 世界阶段验收

世界动作分派先执行原辅助请求接受表，再选择携带／投掷或既有静止 handler。人物命令立即写入同一权威辅助对象，随后调度器才执行对象阶段；`PoseSource` 只在原跟随阶段写入。`WorldLoop` 在输入阶段完成其接触标志修改后传入局部副本，指针使用 `json:"-"`，不新增持久真值或把缺失场景来源默认为零。

新增固定规则世界链验证：人物拾取绑定、准备动作 14、保持 owner 到动作 3 入场、释放后的飞行和落地，以及没有拾取标志时原子拒绝。每次世界动作阶段前调用原 `BeginOriginalContacts` 清除调度 processed 位；绑定后的最终 attachment 为 192，释放后为 64，未忽略该位。此测试不冒称原完整对局。已有 1,998 原动作阶段、1,998 连续世界更新和 1,997 原准备阶段保持通过。

世界接线日志：`.local/basketball/engine/auxiliary-world-{before,after,race,wasm}.log`。本次没有运行完整比赛，集中门禁由主任务执行。

## 携物移动与 B 抛弧组件扩展

本节先完成独立组件验收，随后按下述世界适配接入新增动作。原 group20 分派证据为 `ev_dec0f416070efb8b82777831be8085b1764776c97a199f46e161599489631eec`（`.local/basketball/rea/auxiliary-movement/analysis/`）。

恢复 `$21` 行走、`$22` 跑动、`$23` 急停、`$24/$25` 立即起跳、`$26` 落地、携物 `$0F` 下落，以及地面／空中 A、B 投掷相关 `$27..$2C`。`$24/$25` 不插入空手蹲跳：原函数先清附着／processed 位，立即设置速度后执行当次物理。B 链使用道具动作 2；空中 B 先进入道具 15，再进入 2。道具 2 读取旧 owner 朝向／高度和共享选筐弧标志，经原弹道计算后才释放 owner。

`auxiliary-movement.json.gz` 保留 **4,672 次**原 E18B 完整动作调用，覆盖九方向、真人／电脑、不同能力等级、持有对象 4／5／无对象、地面／空中、进入／持续、跳跃装备和附着投掷。`auxiliary-lob.json.gz` 保留 **308 次**原道具 2 调用，其中四条飞行到落地的序列只初始化一次状态。道具 15 补入原辅助探测后，`auxiliary-carry-probes.json.gz` 总计 **152 次**。这些均为隔离原函数证据，不计作自然完整比赛。

物理绑定额外回归曾捕获：跳跃清附着若在 handler 返回后才应用，原 motion binding 仍会把人物位置贴到支撑物。现组件在调用原物理前，以事务局部 attachment 覆盖向绑定传递清除结果；物理返回的最终 attachment 再作为命令输出，未修改公共物理函数。先失败日志为 `auxiliary-jump-binding-before.log`。

### 新增接线数据

- `AuxiliaryCarrierContext.Support`：附着对象的真实 Body／Pose／Kind；缺失时报错。
- `CarriedPlayer *int8`：空中投掷分支读取的原 `$0569`。必须明确提供，不能把未知来源默认为 −1；人物被他人搬运的 `$8C33` 分支仍未恢复。
- `EnhancementEnabled/Team/JumpEnhancement *uint8`：原 `$EC31` 的跳跃装备查询。JumpEnhancement 来自 `$06E6+3*character`，现数据在 `WorldLoopContext.JumpEnhancements`，不能误用 `$06E5` 的 `Actions.Enhancements`。
- 新输出 `ChangeShotCycle/ShotCycle`、`ClearSpecialShot`、`ChangeAimWindow/AimWindow`、`ChangeAttachment/Attachment`、`Revisit` 和 `SoundIDs` 必须在原阶段提交。`ChangeAttachment` 给出物理后的最终值，不应再盲目用旧附着关系覆盖。
- 道具 2 初入要求 `AuxiliaryContext.Launch`，其中 `ArcFlags` 来自当前共享 `SelectedHoop` 对应的原弧标志。返回 `AuxiliaryStep.LaunchPlan` 只写共享 Target／ArcHeight，不修改 SelectedHoop 或 BallLanding。弹道通过 `court.launchPass` 读取已有当次斜向传球上下文，不自行填第二席 X 小数。

`$2D/$2E` 的闪避及 `$2F` 双方联动没有扩展到本组件；同时持球／持道具的跑动蓄势和搬运人物空中投掷仍未恢复；Falling 的共同持有分支已由下述世界接线补齐。`world_human_input.go` 仍有持道具按键分支的明确未恢复错误，新增动作组件本身不等同于真人完整携物控制已可用。

本轮日志为 `.local/basketball/engine/auxiliary-movement-{before,after,race,wasm}.log`，道具抛弧先失败为 `auxiliary-lob-before.log`。新增提取脚本为 `probe_auxiliary_movement.py`、`probe_auxiliary_lob.py`。本轮未改 `world_actions.go`／`world_loop.go`，未重复完整比赛门禁。

## 携物移动世界适配

`world_auxiliary.go` 集中构造携物上下文与提交命令，世界分派现覆盖已恢复的 `$20..$2C/$4E`，以及实际持有辅助物的 Falling。支撑资料由当前权威对象生成，包含 Court4 已变化的道具 Kind；EC7B 的 owner-follow 与 EEFF 的 attachment 支撑分开传入，不能互相代替。

`WorldLoop` 在动作前传入 `JumpEnhancements` 的局部副本，原来源仍为第三装备槽 `$06E6`；与拾取标志一样，该指针使用 `json:"-"`，不保存第二份设备状态。空中分支直接读取当前 `CarriedPlayer`。适配提交 ShotCycle、特殊标记清除、AimWindow、物理后的 attachment、Revisit 和音效；跳跃动作声音按原顺序排在本次 motion 产生的声音之前。

辅助对象的动作 2／15 已加入同一分派，抛弧标志取当前 SelectedHoop 对应的挂载标志（或原其他对象弧标志），未提供有效选择时不默认填零。LaunchPlan 只更新共享 Target／ArcHeight，SelectedHoop 与 BallLanding 保留原值。

`world_auxiliary_movement_test.go` 将 4,672 个原人物调用经过真实世界人物适配逐项比较，并将 152 个辅助调用与 308 个抛弧调用经过真实世界对象适配比较完整状态。固定规则测试另检查动态 Kind14 支撑与 owner／attachment 指向不同对象时的位置来源。这些测试不冒称完整对局。世界接线先失败及结果保存在 `.local/basketball/engine/auxiliary-world-movement-{before,after,race,wasm}.log`。

真人持物输入入口由另一个独立任务恢复，本次不修改 `world_human_input.go`；完整比赛仍由主任务集中验收。

接线审阅另恢复原 `$9749` 的同时持物／持球 Falling 分支：原指令探测明确设置两类归属，核验球命令与人物状态。`AuxiliaryCarrierContext.HasBall` 取当前权威 Holder，传入 motion binding 避免误释放球；`AuxiliaryCarrierStep.Ball` 传递原持球动作与地形命令。世界适配一起提交该命令。先失败与通过日志为 `auxiliary-world-ball-ownership-{before,after}.log`；原 4,672 调用集合中的相关分支同时比较球状态。
