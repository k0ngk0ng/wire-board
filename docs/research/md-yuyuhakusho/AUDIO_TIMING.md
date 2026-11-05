# 声音队列的原生时基与可重入阶段

已恢复 `219ac` 和 `219ea` 的四个独立 Go 阶段，覆盖延迟请求、驱动忙标记、重入积压与补执行。原生竞态和 Node／Wasm 的 7 项检查全部通过。新原版观察同时证明：消费确实会发生在人物和场地回调内部，不能直接把已有 `ConsumeSoundQueue` 放到世界帧尾就视为完成音频接入。

本阶段没有修改世界、会话、优先级消费器、媒体目录或运行时桥，也没有实现驱动合成或播放。机器索引见 [sound-scheduler-validation.json](../../../games/yuyuhakusho/engine/testdata/sound-scheduler-validation.json)。

## 唯一状态归属

| 原位置 | 语义 | 当前归属／接入约束 |
| --- | --- | --- |
| `A030.byte` | 世界待提交声音 | 现有 `ChuWorldState.PendingSound`，不能当作 `F562` |
| `F560.word` | 延迟计数 | 新 `SoundSchedulerState.Delay` |
| `F562.byte` | 延迟声音编号 | 新 `SoundSchedulerState.DelayedCode` |
| `F563.byte` | 重入积压 | 新 `SoundSchedulerState.Backlog`，保留 byte 回绕 |
| `F564.byte` | 驱动忙标记 | 新 `SoundSchedulerState.Busy`，保留原 byte 语义 |
| `F807.byte` | 暂停请求／确认 | 现有 `Clock.AudioPause`；不能另存到 scheduler |
| `F80A..D` | 四槽请求队列 | 现有 `ChuWorldState.SoundQueue`；helper 只收发临时值 |
| `F800.byte` | 当前优先级 | 优先级消费器已有临时 `SoundPriorityState.CurrentPriority`；世界接入仍需唯一持久 owner |
| `F809.byte` | 选中待分派编号 | 同上，使用 `SelectedSound` 临时视图；世界接入仍需唯一持久 owner |

驱动 `F80E` 的声部组游标、fade／tempo 状态、活动行、Z80 标记和硬件状态不属于新 scheduler。静态和自然采样都保留它们的边界，但不能以恢复四个 scheduler 字段宣称完整驱动已恢复。

## 原控制流与 API

`219ac` 先处理延迟：计数非零则只减 1，本次不提交；计数已为 0 且编号非零时调用原请求器，成功才清编号。随后检查 busy；非零则积压加 1 并返回，否则设置 busy=1、调用驱动，返回后清 busy。

外层主循环 `3d8` 调用 `219ea`。积压非零时设置 busy=1、调用一次驱动，再清 busy、减当前积压并回到条件判断。因此补执行期间发生的新重入必须保留，不能在驱动返回时用进入前的积压覆盖它。

正式接口位于 [sound_scheduler.go](../../../games/yuyuhakusho/engine/sound_scheduler.go)：

```go
BeginSoundInterrupt(state, queue) (nextState, nextQueue, drive)
FinishSoundInterrupt(currentState) nextState
BeginSoundCatchup(state) (nextState, drive)
FinishSoundCatchup(currentState) nextState
```

开始和结束之间由调用者执行驱动，或先执行嵌套 IRQ 阶段。helper 不生成聚合 `DueTicks`，不决定世界帧中的插入位置，不拥有队列或暂停副本，也不执行 CPU／RAM VM。

静态来源复用 [arena-a-consumers.txt](static/arena-a-consumers.txt)、[first-control.txt](static/first-control.txt)、[sound-queue-callsite.txt](static/sound-queue-callsite.txt)、[audio-loop-sequencer.txt](static/audio-loop-sequencer.txt) 和 [world-frame-irq.txt](static/world-frame-irq.txt)。它们来自 direct Ghidra；REA 不支持本任务的 raw 68000，不应称为 REA 原生反编译结果。

## 自然时序证据

新被动观察器冷启动执行已有 5,500 步合法输入。每一步的完整 RAM、视频与 PCM 摘要均与已有未观察内核控制组相同，重现原 109 次请求和 107 次非 `FF` 分派。另增加在 3000／3040 步按 Start 的合法暂停流，新的未观察与观察内核各执行 3,602 步，RAM、视频和 PCM 全部相同。

| 项目 | 既有输入流 | 新暂停输入流 |
| --- | ---: | ---: |
| 视频步骤 | 5,500 | 3,602 |
| 闭合 scheduler 调用 | 5,192 | 3,295 |
| 实际 driver tick | 5,195 | 3,298 |
| 队列消费 | 108 | 77 |
| 非 `FF` 分派 | 107 | 76 |
| driver tick 位于对象回调内 | 3,163 | 2,042 |
| consumer 位于对象回调内 | 83 | 62 |
| 最大记录 IRQ 嵌套深度 | 3 | 3 |
| 观察到的最大积压 | 0 | 0 |

两流合计 16,974 条 `219ac` 开始／结束阶段投影、185 个真实消费器调用已分别与 Go helper 和既有优先级消费器逐字段比较。consumer 中，既有输入有 62 次发生在 `12668` 人物回调中、8 次发生在 `20918` 场地回调中；暂停输入分别为 49 次和 5 次。保存的中断 PC 包括 `15d92`、`17fdc`、`1ebca`、`20912`、`20ad8` 等回调内部位置。

`audio-timing-frame-end-red.json` 保留真实失败：两流有 145 个回调内消费反例，直接否定「全部消费只在对象回调结束后发生」的假设。没有通过扩大 mask 或移动原期望来消除这些反例。

driver 也不是无条件每视频步骤恰好一次。两流都在第 9／330／1513 步观察到两次 tick，其他初始化步骤可能没有 tick。原始 CPU cycle 保留为证据；既有流范围为 6,837～373,005，不能直接作为 PCM 样本位置或独立 Go 运行时的生产回放表。

暂停流中，第 3001 步 driver 入口读到 `F807=1`，原路径确认后为 `FF`；接着有 39 次 `FF` 入口；第 3041 步读到 `80`，恢复路径清为 0。所有非零暂停入口都绕过队列消费和分派。该证据与平台静音后的「不补播旧媒体」是不同层次，不能混作一个状态。

marker 同时保存原始与 24 位归一化 PC、SP、SR、CPU cycles、完整视频步号、IRQ 保存的中断 PC／SR、对象回调身份和严格序号。`world_pass` 只是最近一次对象轮次的序号，不以其非零值推断正处于对象轮次。

## 私有边界与连续原调用

私有参考在同一第 2,400 步检查点启动，恢复总是补齐 `refresh_cycles`。`F807=FF` 让实际驱动按原路径返回而不合成，隔离 scheduler 自身的写入；没有替换驱动指令或伪造消费者。

共 1,592 个原调用阶段，其中 1,332 个边界、260 个单初态连续阶段。覆盖：延迟 `0/1/2/FFFF`、延迟编号 `0/58/82/85/FF`、空／混合／受特殊请求占据的队列、busy `0/1/FF`、积压 `0/1/2/3/7/FE/FF`，以及四条在开始和结束之间调用原 IRQ 阶段的连续流。后者是明确的私有阶段组合，不冒称自然硬件 IRQ 重入。

每次比较完整 64 KiB 前后差异，除声明的 scheduler、队列、驱动组清零及精确私有调用栈区之外，未知写入为 0。开始／结束阶段在原指令边界停下，补执行结束覆盖当前积压为 0 时递减到 `FF`，验证没有用进入前副本覆盖嵌套改写。

自然流未观察到非零积压，也没有非零延迟请求。因此自然输入只证明实际时序和消费位置；延迟、重入回绕与补执行由私有原边界和单初态阶段流另行证明。

## 测试先行与复跑

[失败清单](AUDIO_TIMING_FAILURES.md) 先于新采样器与 helper。第一版采样器的私有 SP 检查失败，因为停点 `3d8` 自身随后执行 JSR；第二版改为在停点记录 SP，并新增四阶段原调用。原脚本、真实失败和修复谱系保留。两个自然数据集来自未改的第一版；它们保留各自 producer SHA，未因私有检查修复而重采。

五项新测试先留下缺实现 red，再冻结，之后才实现 helper。自然断言也在实现前加入并留下第二份缺符号 red。最终冻结测试源码在 red、native、Wasm 间未修改；旧优先级两项测试一起复验。

独立 Go 验证只依赖已保存夹具：

```sh
TEST_TELEMETRY_DIR="$PWD/.local/go-telemetry" \
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -json -count=1 ./games/yuyuhakusho/engine \
  -run '^Test(SoundScheduler.*|SoundQueue.*)$'
```

Wasm 的最小环境与命令见 `engine/testdata/sound-scheduler-wasm-command.json`。重新检查现存原时序数据并导出临时语义投影：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/analyze_audio_timing.py \
  --out .local/md-yuyuhakusho/audio-timing-projection-review
```

用 `--expect-frame-end-only` 会按预期返回非零，复现 145 个回调内消费反例。重新采私有原调用使用 `capture_audio_timing.py --mode private --out <工作区内新目录>`。旧自然观察器完整源码保留为 `engine/testdata/audio-timing-capture-v1.py.txt`；设置 `PYTHONPATH=scripts/yuyuhakusho` 可直接以 Python 执行该保存版本并选择新目录。正式 ROM、core 和旧研究媒体均为只读输入。

## 世界接入前仍缺少的证明

当前 `StepChuSessionFrame` 不是原 CPU 的逐周期执行器。现有证据没有恢复一个能从独立规则状态推导所有 IRQ 插入点的周期预算或等价调度算法。把采样到的原 PC／cycle 时间表带入新对局、每帧末尾随意消费一次、或引入 CPU／RAM VM 都不解决这个缺口。

接入必须继续核对：规则音效生产阶段与视频音频时钟的关系；回调内消费改变队列后的后续请求；正常驱动 housekeeping 对优先级、分派与声部抢占的影响；暂停确认、恢复、控制命令与有限媒体起播原点。整个独立驱动、精确混音、BGM 循环和平台实际播放尚未由本阶段证明完成。

本分工清理 14 个重复／临时路径，共 4,367,405 bytes；原私有采样与离线语义投影的第二次导出均与规范 gzip 逐 byte 相同。验收索引绑定[一致性记录](../../../games/yuyuhakusho/engine/testdata/sound-scheduler-determinism.json)与[清理范围](../../../games/yuyuhakusho/engine/testdata/sound-scheduler-cleanup.json)。
