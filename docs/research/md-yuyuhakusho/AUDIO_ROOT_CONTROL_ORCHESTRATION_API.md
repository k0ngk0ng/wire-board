# Root 规则阶段协调接口

`AdvanceRules(root *State, owners RuleOwners, input RuleInput, hooks RuleHooks) (RuleOutput, error)` 完成一个规则 tick，不持有长期状态或时钟。caller 在成功后提交全部 detached owners；失败时丢弃全部 detached owners。score 自身事务不在失败时提交，外部 hook 已产生的副作用不能由本包撤销。

| 输入 | 类型与约束 |
| --- | --- |
| `root` | 非空 `*State`，保持已有五项 Root 字段的唯一 owner |
| `owners` | `RuleOwners{Score *score.Driver, Queue *[4]uint8, AudioPause *uint8}`；三项均须非空并已 detached |
| `input.Priorities` | 不可变 `engine.SoundPriorityTable`，按原 D→A 顺序消费 queue |
| `input.Idle` | 瞬时 `RuleIdleProjection`；control mode、FM／PSG delta、duck、signal、FM3 mode 与 8 项 offsets 均须为零，不新增持久字段 |
| `input.RequireCost` | 默认为 `false`；为 `true` 时在任何效果前返回 `ErrRuleCostUnavailable` |

`RuleHooks.Operation(RuleOperation) error` 和 `RuleHooks.Score.Operation(score.Operation) error` 均必填。Root consumer 同步执行 Root 的实际 chip 效果，score consumer 同步执行 score 的实际效果。纯规则验证须显式提供空 consumer。`ReadDAC` 直接委派到 score 的现场反馈读取点，不能复用 tick 入口反馈。

`CaptureLevels() (PauseLevels,error)` 只在 pause capture 点读取实际 chip 的 24 个 TL。`DispatchExternal(id uint8) error` 处理实际接受的非 score 请求；需要分派而未提供服务时返回不支持。默认只接受来源审计的 139 个候选。139 候选外、但已静态证明维持 control idle 的 ordinary／完整 primary 测试域，须通过临时 `AcceptDispatch(id) error` 显式接受；`7F` 无法通过这个扩展进入。hook 不保存新 candidate 表。

正常阶段为 `Gate→ControlIdle→Tempo→Fade→Queue→Dispatch→Voices→Return`。协调器先取得唯一 score 租约，首个 Root callback 仍是 Gate；建立事务不推进 tempo。Root phase、pause live reads／writes、capture／commit／release、fade counter writes、queue consume、selected writes、dispatch 与 chip writes 都有具名语义效果。callback 在相应效果前同步执行。

Gate 按原分支读取 live pause 最多三次。pause start／hold／resume 都直接结束本 tick，仅通过 `FinishSuspended` 使 `Ticks` 加一。start 写 `FF` 后捕获 24 个 TL，完成 24 次 FM mute 与四次 PSG mute 后提交 pending；resume 写 0 后恢复 24 个 FM TL，再释放 pending，不添加 PSG restore。缺少必要 pending 信息时返回不支持。

queue 只在 Queue 阶段构造临时 `engine.SoundPriorityState`，调用 `ConsumeSoundQueue` 后立即拆回各唯一 owner。dispatcher 读取 selected 并先写 `FF`；`01/05` 按 remaining→steps→score DAC flags 执行，仍在当前 tick 扫描 voices。04 按 YM `27=0`→`RuleBulkClear`→selected=`FF`→六次 keyoff→24 次 TL→四次 PSG 执行。

`RuleBulkClear` 的字段为 `HeaderLongs=16,VoiceLongs=216,MusicOnly=false`，表示 64-byte header 与全部 18 voices 的来源边界。没有 `VoiceCount` 字段。该效果不发布逐 long 中间态；score 全部清理由 `ActionResetScore` 在同一规则 bulk 中执行一次。music reset 的同一效果为 `HeaderLongs=16,VoiceLongs=120,MusicOnly=true`，由 score 自身清前 10 个 music voices，保留 primary／secondary。

score 的 `OpPrioritySave` 在真实节点保存临时 priority latch；`OpHeaderClear` 清 Root／World header 字段；`OpPriorityRestore` 恢复 latch；`OpSelectedReset` 写 `FF`。普通 `OpPriorityClear` 只在真实 byte-clear 点执行，不合并为 tick 末 bool。`OpQueueReplace` 只在真实节点写唯一 `Queue[0]`。

`RuleOutput` 只有 route 和诊断 score 输出，不含 cycle 或 cost。输出 intents 与 `ClearPrimaryPriority` 已由 hooks／节点消费，不能在 tick 末重放。完整定价、逐 long 中断可见性、异步 IRQ 和未接入的普通 PCM 接受／生命周期必须继续由各 owner 明确补齐。
