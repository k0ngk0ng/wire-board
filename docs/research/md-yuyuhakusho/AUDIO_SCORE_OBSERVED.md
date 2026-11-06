# 音序的有序操作与阶段反馈

本版在同一套 typed score 推进中新增 `AdvanceObserved`，同步报告临时语义操作，并在 DAC 仲裁与声像重定向的各自阶段即时读取外部反馈。旧 `Advance`、`State`、schema 2 与既有 42 条原版流保持兼容；不把这版接口表述为完整 master-cycle 已恢复。

原生 `-race` 与 Node／Wasm 各 138 条测试记录全部通过，`go vet` 通过。每端分别完成旧接口与新接口各 42 流、126,141 tick、1,504,150 个有序 intent 的一致对照，新接口报告 18,855,491 个临时操作。完整来源与结果见 [audio-score-observed/validation.json](../../../games/yuyuhakusho/verification/audio-score-observed/validation.json)。

## 接口与提交边界

`AdvanceObserved(ObservedInput{SoundID}, ObservedHooks)` 返回原有 `Output`。`ObservedHooks.Operation` 必须提供，`ReadDAC` 在实际需要反馈时调用；可选 `Parser` 为不可变 sidecar，`MaxOperations` 限制本次回调预算。默认上限为 262,144 个临时操作。操作不进入 `State`，调用之间不保留历史、原 PC、原 bytecode、原栈或另一份 queue／priority。

`Operation` 含版本、连续 sequence、routine／branch、voice slot、逻辑事件位置、当前 flat intent offset，以及最多两条有序 writes。逻辑位置使用 score／voice、prefix／cycle 和 event index；`Entry` 只在现场由既有 cursor 转换得到，用于区分 continue、from-prefix 和 cycle-wrap，不新增持久圈数。

Root 字段和 chip effect 在相应 hook 返回成功后继续。callback、反馈、预算或结构校验失败时，score 不提交部分状态或输出，且不再调用后续 hook；外部副作用不可能由 score 自动回滚。

调用者必须先创建 **detached 的全 owner transaction**：用 `Restore` 建立 working score，同时复制或构造 working bus／chip／PCM／Root 字段 owner。hooks 只访问这些 working owners；所有阶段及最终校验成功后一起提交。不得在 authority score 或硬件 owner 上试跑后再声称已经原子回滚；即使 `AdvanceObserved` 成功，后续整个事务校验失败时也应丢弃 working score。

## 顺序和反馈

| 节点 | 语义 |
| --- | --- |
| Enter／Leave 与 routine／branch | 标记当前实际高层阶段；没有根据 Go 函数调用自动附加 JSR／RTS 价格 |
| VoiceScan／Counter／Parameter | 报告活动、等待、gate、modulation、pan 和参数分支，数据来自当前 score 状态 |
| Claim／Restore | 在覆盖、forced-rest 和恢复当前音色／noise 之前报告 |
| PriorityClear | 原 byte-clear 或 bulk reset 的 Root 字段 effect；两者由 branch 区分，不能重复收费 |
| PrioritySave／PriorityRestore | Root 在 detached 事务中持有临时 latch；score 不复制 priority，也不硬编码恢复为 0 |
| ChipWrite／QueueReplace | effect 前给出完整 typed write 与准确 offset |
| PSGPitchWrite | 一次通知包含 latch／data 两条原有 flat intent，组内不重复通知，避免把共同 RTS 算两次 |
| FeedbackRead／FeedbackResult | 在相应实际读取阶段同步查询，并报告 blocked／accepted／rejected／redirect 分支 |

`ReadPanRedirect` 对应 channel-six pan 的 mailbox 非零测试；`ReadBlockedPriority` 对应 DAC note 提交前的 high-bit 与优先级测试。provider 可以在 blocked 时跳过原本不会发生的 priority 读取。两类请求不复用 tick 入口快照：例如 pan 读取之后的 YM 服务改变了外部 mailbox，后面的 note 仲裁必须看到新状态。

已有 `Output.ClearPrimaryPriority` 继续兼容旧逻辑消费者，但时间层必须使用真实节点。在 `music82-a7` 第 80 步，两个 primary 声部均停止，原清理点分别位于 intent offset 12 和 39；不能合成 tick 末 offset 40 的一次清理。初稿测试漏掉第二个停止点，真实 red 和原 routing／write 证据保留，新的精确期望增加为 `[12,39]`，没有删除真实写点。

音乐清理依次报告 StopPrimary clear、MusicReset 的 save／bulk clear／restore。bulk-clear 节点仅提交 Root 字段效果，不能再调用 byte-clear 服务附加一次价格；save／restore 同样有自己的原语义。

## 独立 parser-work 资源

[audio-score-parser-work.json.gz](../../../games/yuyuhakusho/data/audio-score-parser-work.json.gz) 独立绑定冻结 score 的解压 JSON SHA-256 `735d012d3d3140e76f8f756f0ca757bdc4b8deeb5a66750dd4610e41e06af830`，不改变 schema 2 或 payload owner。`DecodeParserMetadata` 核对完整逻辑键、Entry 变体、严格有限的 Work 语法、资源上限及 catalog 身份。

资源含 39,045 个事件键、115 条声部，单变体最多 12 Work。内容只有 token／参数读取角色、有限 jump／call／return、loop first／taken 分支、duration probe／scale 与 pitch／commit 站点，没有源地址、原控制字节、跳转目标或周期价格。score 仍由原 typed event 推进，绝不通过 Work 执行原控制流；Work 仅用于通知和后续独立成本层。

有 8 个 cycle 首项的首次进入与重复入口不同。`EntryFromPrefix` 与 `EntryCycleWrap` 选择对应变体；未知的手工 cycle0 continue 历史明确拒绝，不通过增加原 counter／stack 影子补齐。

Work 不会全部塞在 note 前执行。通知分为 prelude、pitch、early-commit、duration 和 final-commit：FM pitch 站点在 keyoff 后，PSG rest 的早期 commit 在 keyoff 前，duration probe／scale 在其实际语义位置。source 仅为 cleanup 诊断而暂写的旧 wait／last duration 已在 typed 规则中消除，不成为另一份持久 owner；相关原站点和读取工作仍逐项记录。

未提供 sidecar 时发送 `ParserUnavailable`。instrument 寻址、envelope 原控制和动态 body 等未恢复结构通过 `ResourceUnavailable` 标记。成本消费者必须对缺失语义返回 Unsupported，不能计为零。该资源声明 `coverage="parser-structure-only"`、`unpriced=true`；它不是原指令解释器或 oracle 时间表。

原研究和有限 flow 的逐读取对照见 [AUDIO_SCORE_FLOW_RESULTS.md](AUDIO_SCORE_FLOW_RESULTS.md) 与 [AUDIO_SCORE_FLOW_EVIDENCE.json](AUDIO_SCORE_FLOW_EVIDENCE.json)。新 Go 层已按原 23 个定义、122,810 tick 核对 199,477 个 token／lookahead read、26,130 个 control、127,691 个 early／final commit、loop 3,388、jump 942、call 263、return 246；各步的 score 与 flat 输出仍与旧推进一致。

## 时间层边界

`Tempo` 入口的临时 `TempoObservation` 给出 Enabled、Reload、ActiveMusicMask、Period 和 Remaining。reload mask 保留原 10 个 music slot 的位置，非 reload 为零。时间服务的 `CallTempo` 已包含自身真实 caller 与 RTS，后续 counter／scan／Leave 只是语义通知，不能重复计费。

Control／Fade 字段由 Root owner 管理。正常原顺序是 Control、Tempo、Fade，再检查 queue；本版不把缺失的 Root 非 idle 行为假定为零成本，也不新增一份 pause／duck／fade 状态。各服务的 caller、leaf 和返回成本由 [SCORE_CLOCK.md](SCORE_CLOCK.md) 合同及已验证服务决定，不能由 Go 调用树推断。

Root pause／IRQ、动态 body、完整资源服务、独立 PCM cadence、刷新边界、总混音和平台接线仍需相应 owner 验收。能够输出正确的临时操作，不等于已预测任意新输入下的完整 tick 时刻。

## 先行失败与复核

[失败清单](AUDIO_SCORE_OBSERVED_FAILURES.md) 和新增测试先于接口实现。缺 API、缺 tempo work、缺 parser 接口均保留真实编译 red；旧 return-only 协议不能表达中间 priority 写点的契约反例另存，不改旧逻辑证明。新测试验证动态反馈、准确顺序、group span、外部 detached 边界、回调错误、预算、不可变数据及完整原版轨迹。

正式结果与冻结来源由新的 `verification/audio-score-observed/` 记录，旧 `audio-score-driver`、原 fixture 和 `92f27271` 证明保持历史身份。重新提炼 sidecar 仅使用已核验研究输入：

```sh
python3 -B scripts/yuyuhakusho/export_score_parser_work.py \
  --out .local/md-yuyuhakusho/score-observed-review/parser-work.json.gz
```

该命令不运行新 CPU，也不复制音乐或普通 PCM payload。临时导出与正式 sidecar 必须逐 byte 相同。

在 worktree 根目录复跑当前完整验证，输出留在工作区：

```sh
nice -n 10 python3 -B scripts/yuyuhakusho/verify_score_observed.py \
  --mode native --out .local/md-yuyuhakusho/score-observed-review/native
nice -n 10 python3 -B scripts/yuyuhakusho/verify_score_observed.py \
  --mode wasm --out .local/md-yuyuhakusho/score-observed-review/wasm
python3 -B scripts/yuyuhakusho/verify_score_observed.py \
  --mode vet --out .local/md-yuyuhakusho/score-observed-review/vet
```

脚本拒绝覆盖旧 `audio-score-driver` 结果，先核对保留的 State／旧测试／catalog／42 fixture，再记录当前源码、命令、工作区内 cache／telemetry／tmp 和完整日志；运行期间源变更会拒绝本次结果。本轮只删除逐 byte 相同的第二份 sidecar 导出 199,277 bytes，原来源、真实失败、错误假设和旧证明均保留。
