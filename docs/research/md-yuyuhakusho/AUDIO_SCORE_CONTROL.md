# 比赛 score 控制动作与分阶段事务

本版为 Root 比赛控制提供零 tick 的 score 动作，以及同一 working score 的分阶段推进。Root 仍独占 priority、selected、fade、queue、pause、scheduler、PCM 和 chip；score 不保存这些字段副本，也不执行本轮闭包外的 duck／manual menu profile。

原生 `-race`、实际 Node／Wasm 各 168 条测试记录全部通过，`go vet` 通过。新旧接口各 42 条原版流、126,141 tick、1,504,150 个有序请求保持一致；94 个独立控制动作窗口、parser 原读取计数和事务重入拒绝均通过。结果与冻结 SHA 见 [audio-score-control/validation.json](../../../games/yuyuhakusho/verification/audio-score-control/validation.json)。

## 控制动作

`Driver.ApplyMusicControlObserved(action, hooks) -> (Output, error)` 不推进 Ticks、tempo 或正常 voice tick。可用动作如下：

| Action | score 效果 | 不属于本动作的效果 |
| --- | --- | --- |
| `ActionStopMusicDACFlags` | 清 music DAC 的 Active／NoteOff／Claimed／Tie 四个 flags，保留 owner、参数及事件位置 | 不重启或停止独立 PCM，不清整条 voice |
| `ActionFadeMusicStep` | 仅遍历活动 music FM1..6／PSG1..3，按原 byte／signed 阈值修改 BaseLevel、必要时清 Active，或发当前 level／base attenuation 写 | 不推进 NoteWait、gate、envelope、modulation、pan、tempo 或 Tick |
| `ActionResetScore` | 清 Tempo 和全部 18 条 voice，保留诊断 Ticks | 不清 Root 字段，不发原 04 的 YM27／六 keyoff／24 TL／四 PSG，不停止 PCM |

FM 的 BaseLevel 按 byte 加一；结果为 signed 负值时只清 Active，否则调用已有 level 服务。PSG 加一后达到 16 时只清 Active；未达到时直接使用 base attenuation 服务，不调用 envelope.Next。FF→0 的 byte wrap、note-off／claim／tie-gate 抑制及 FM loaded algorithm／carry 仍遵循原语义。

Root 的 pure fade counter 决策先提交其有序 counter writes，再映射 score Action。01／05 映射 StopMusicDACFlags，正常 fade 步映射 FadeMusicStep，terminal 请求进入 Root 04 编排。04 的硬件／Root bulk 顺序不能由 score 动作重复施加。

## 阶段事务

```go
tx, err := workingScore.BeginTickObserved(hooks)
err = tx.AdvanceTempo()
// Root control/fade, queue consume and accepted dispatch happen here.
err = tx.ApplyMusicControl(action) // Optional; only after Tempo, before Dispatch.
err = tx.Dispatch(acceptedScoreID) // 0 for a Root control/PCM/no-score dispatch.
err = tx.AdvanceVoices()
output, err := tx.Finish()
```

`AdvanceTempo`、`Dispatch`、`AdvanceVoices` 各执行一次。音乐初始化原本直接退出 driver tick，因此 Dispatch 标记跳过 voice scan，随后 `AdvanceVoices` 只闭合该阶段，不额外推进新音乐。Root 不得先在外部推进 Tempo 后再调用完整 `Advance`。

暂停路径可以在刚 Begin、尚未推进 Tempo 时调用 `FinishSuspended`，只计一次实际 tick，不改变 score 参数或推进声音轨道。pause byte、pending 24 TL 和硬件静音／恢复由 Root／chip owner 管理。

事务持有一个临时 working score，Finish 前不修改原 driver。`tx.Snapshot() -> (State,error)` 给出当前工作分支，`Abort()` 丢弃。Driver lease 拒绝另一个 Begin／Advance／局部 mutation；阶段乱序、重复调用、错误或关闭后使用均拒绝，不能部分提交或多计 Tick。该 lease 与执行保护仅为临时 API 协调，不进入 `State`，也不是另一组持久 voices。

同一 Tx 的阶段 callback 也不能递归调用其阶段或 Finish。独立审阅曾实际复现 Tempo 重跑和 Ticks 从 2 增至 4；7 项 red 后增加临时 running guard。重入或 callback Abort 会取消外层，并保持 lease 直到该外层调用退出，不能在同一 callback 里趁错误提前释放后劫持 Driver。只读 Snapshot 仍可在 callback 中调用，返回值不 alias 工作状态。

Root coordinator 必须在任何用户 hook／gate callback 前先取得 score lease，防止 score 尚未开始但 Root 已递归推进。`OpEnter Tick` 只是语义 transaction 起点，不等于应重复执行 gate 或原 caller 成本。

旧 `Advance`／`AdvanceObserved` 封装同一组阶段实现，旧 State 结构和资源保持兼容。所有外部 hook 仍必须作用于 caller 的 detached 全 owner transaction；score 的局部原子性不等于能回滚已经写入 authority bus／chip／PCM 的副作用。

Root 若已经在 hooks 中按真实节点消费了 Intent／PriorityClear，最终 `Output` 只作为诊断结果，不能在 tick 末再次重放。兼容 bool 不能代替有序 Root 字段 effect。

## 音乐 reset 的 Root effect

原音乐初始化只清前 10 条 music voice，不清保留的 primary 参数；04 的 ResetScore 则清全部 18 条。两者不能混用。

音乐清理的 Root 字段节点现明确为：

`PrioritySave → HeaderClear(16 long) → score 前10／Tempo 清理 → PriorityRestore → SelectedReset(FF)`。

StopPrimary／单轨停止的 PriorityClear 仍是原 byte-clear。HeaderClear 是 64-byte header 的 Root owner group，不再暗含成一个独立 PriorityClear 让成本消费者重复收费；136-long 总 music bulk 的定价与 Root 各字段 effect 由外层按原写点编排。score 本身不复制 queue、fade、pause 或 slot-choice。

## 原版动作窗口

新 [control-oracle/native-actions.json.gz](../../../games/yuyuhakusho/audio/score/testdata/control-oracle/native-actions.json.gz) 从已保存的 Root native 边界提炼 94 个动作窗口：StopDACFlags 2、FadeStep 88、ResetScore 4。88 次 fade 的 968 YM 与 42 PSG 写有原序列期望；ResetScore 的期望 IO 为空，原 FullClear 31 YM＋4 PSG 另列 Root 硬件段。

测试由合法 `score.New`、实际 accepted score dispatch 和早先动作重建 before，不把原 PC、loop stack 或 event cursor 注入 GoState。原 before／after 只比较 Tempo 与 voice 字段，排除 Root fade／pause 等 owner。Ticks 和 Go 事件位置的保留另用动作前后 Snapshot 验证。

Stop 原观察只直接采到变更后的 DAC row；其他 score 字段来自原 before，并由该原单 flags store 以及 row 参数逐 byte 不变证明。它不是完整 after 直接采样；原样本的其他三个 flag 本来为零，四 flag 全清还需静态源与单独边界检查。自然 fade 窗口没有达到 FM 127 阈值，也未覆盖 claimed-fade 或非零 tempo；这些限制与边界测试来源见 [AUDIO_SCORE_CONTROL_ORACLE_RESULTS.md](AUDIO_SCORE_CONTROL_ORACLE_RESULTS.md)。

先行失败清单与缺 API red 见 [AUDIO_SCORE_CONTROL_FAILURES.md](AUDIO_SCORE_CONTROL_FAILURES.md)。本轮不修改历史 Observed／42-flow golden，也不把动作规则通过表述为 Root 完整 IO、异步时基或实际混音已经完成。

## 集成文件与复跑

生产变更限于 `audio/score/control.go`、`transaction.go`，以及已有 `driver.go`、`observed.go`、`state.go` 的阶段封装、Root effect 枚举和临时 lease。持久 State 结构、catalog、parser 资源和旧测试／42 fixture 保持原样。Root 集成直接使用这些冻结 API，不需要新原采或扩展菜单成本研究。

在 worktree 根目录复跑：

```sh
nice -n 10 python3 -B scripts/yuyuhakusho/verify_score_control.py \
  --mode native --out .local/md-yuyuhakusho/score-control-review/native
nice -n 10 python3 -B scripts/yuyuhakusho/verify_score_control.py \
  --mode wasm --out .local/md-yuyuhakusho/score-control-review/wasm
python3 -B scripts/yuyuhakusho/verify_score_control.py \
  --mode vet --out .local/md-yuyuhakusho/score-control-review/vet
```

脚本先检查历史测试／资源／42 fixture 和持久 State 布局，再记录当前冻结源码、控制窗口、命令、工作区内 telemetry／cache／tmp 和日志；不得覆盖历史 driver／Observed 验收。外层 hooks 已处理的 Intent／字段 effect 不在最终 Output 处重放。
