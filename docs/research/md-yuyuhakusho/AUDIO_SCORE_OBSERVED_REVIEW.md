# 操作观察与 parser 接口独立审阅

本轮只读审阅 `observed.go`、`driver.go` 和 `parser_work.go`，未修改生产、现有测试、catalog 或原版 fixture。针对事务、同步反馈、priority、别名和 parser 查询运行了 5 项现有小型接口检查，全部通过；运行前后源码、catalog、parser 资源及原 index 的 SHA-256 均一致。结果与命令见 [targeted-positive-command.json](../../../games/yuyuhakusho/verification/audio-score-param/observed-review/targeted-positive-command.json)，日志为同目录 `targeted-positive.jsonl`。

## 结论

没有发现新的高风险事务或反馈顺序问题。审阅确认以下边界：

| 检查点 | 当前行为 |
| --- | --- |
| Effect 前 callback | `emitFor` 先调用 operation，成功后才把 intent 加入输出；PSG pitch pair 只通知一次，`WriteCount=2`，随后保留两条 flat intent 顺序 |
| 失败短路 | 第一个错误锁定在 tick 中，后续 Operation／ReadDAC 不再调用；失败返回空输出，score 原状态不提交 |
| 外部副作用 | callback 已改变的外部 owner 不会被 score 暗中回滚。调用者必须操作 detached owners，仅成功后统一提交 |
| 同步读取 | PanRedirect 与 BlockedPriority 在各自阶段调用 ReadDAC，读取前先通知 OpFeedbackRead；前序 chip callback 可以改变后续读取的外部 mailbox |
| Primary 停止 | 当前轨道 key-off 后发 priority clear，再 forced-rest／恢复当前音乐参数；不是在 tick 末把多次 clear 合并 |
| 音乐 reset | YM27 写后 save，再进入 bulk-clear 分支，随后 restore；与 StopPrimary 的 byte clear 位置区分 |
| 可变值 | operation 中的 Pan 独立复制；writes 为值数组。Lookup 返回新 slice，metadata 的 Entry plan／sites 不由调用者修改 |
| Parser 顺序 | Entry 在 prefix／cycle 边界派生；prelude、pitch、early commit、duration、final commit 位于各自明确 site，没有用旧／新 state 猜消失的原控制路径 |

这 5 项检查分别为 `TestObservedFeedbackReadsAreStaged`、`TestObservedPriorityWritesAreAtOriginalPositions`、`TestObservedErrorsDoNotCommitScoreOrRepeatCallbacks`、`TestObservedRecordsHaveNoMutableOutputAlias` 和 `TestParserMetadataRejectsUnknownAndUnboundData`。本分工没有重复运行全部 42 条流或 122,810 步 parser 对照；完整范围以主分支的最终原生／Wasm 验收为准。

## 已交给主分支修复的拒绝边界

只读审阅发现 empty Work 和合法词汇但错误 pitch family／form 可被构造器接受；主分支另补了必填 bool／additions 为 null 的同类问题。这 6 项由独立 `parser_binding_test.go` 保存真实 red 后修复：拒绝空 variant，按绑定事件／通道核对 pitch grammar，拒绝必填 scalar 的 null。此分工没有重复或改动这些回归与生产实现。

## 保留边界

Operation 是暂时的语义观察，不是 master-cycle，也不是价格表。`ResourceUnavailable` 仍表示尚未恢复的资源工作；消费者不能当成零成本。音乐 reset 的 bulk-clear 分支也不能直接套用 ordinary byte priority clear 的成本。Root pause／duck／fade、完整资源访问工作和具体时基仍由后续 provider 单独验收。

本分工只保留小型验证命令与日志，没有创建浏览器、私有 CPU 采集或临时 Go 源，也没有修改原 42 条 fixture。共享且仍在使用的 worktree Go cache／tmp 未做删除。
