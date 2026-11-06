# PSG raw store 与固定叶操作时序

`audio/scoreclock/psg.go` 已恢复 PSG raw store，以及 single、pitch-pair、mute-all 三个固定叶操作的局部 master-cycle 成本。两份固定 native core 的 138 个声明私有配置产生相同的 192 次有序写入；Go 原生竞态测试与 Wasm 均使用真实 `chips.Interactive`，写点、最终时钟和芯片语义状态与原生结果一致。

## 调用边界

```go
// 只计一次 absolute data-register PSG store，不计返回。
next, err := scoreclock.StorePSG(clock, value, chipOwner, scoreclock.Limits{})

// 从已准备好的 canonical 字节开始，包含一次叶操作返回。
next, err := scoreclock.ServePSG(clock, scoreclock.PSGRequest{
    Kind: scoreclock.PSGPitchPair, Value: latch, Data: continuation,
}, chipOwner, scoreclock.Limits{})
```

`PSGChip` 仅要求 `WritePSG(cycle uint64, value uint8) error`；`*chips.Interactive` 直接满足接口。PSG latch、分频器、音量、噪声和合成状态继续由该外部 owner 唯一持有。`Clock` 是每次调用的输入／输出游标，不在本服务中持久保存。

| 操作 | payload | 固定语义边界 | 无 refresh 时的 master cycles |
| --- | --- | --- | ---: |
| `StorePSG` | 任意物理 PSG byte | 一次 absolute data-register store；不含返回 | 112 |
| `PSGSingle` | `Value`；`Data` 必须为 0 | 一次 absolute data-register store，随后返回 | 224 |
| `PSGPitchPair` | `Value` 的高半字节为 `8/a/c`；`Data` 为 `0..63` | tone latch、continuation 两次 absolute store，随后返回 | 336 |
| `PSGMuteAll` | `Value`、`Data` 必须为 0 | 选择端口，按 `9f/bf/df/ff` 顺序执行四次 immediate store，随后返回 | 532 |

single 包含 volume／keyoff 的 store 与返回；二者之前的字节准备、分支和 caller 的调用成本由调用者另行处理。pitch-pair 也不包含音高计算、channel 映射或 caller 的调用成本。raw store 供语义阶段单独分摊返回成本，不能再当作完整 single 使用。

所有固定操作复用冻结 `service.cost` 的 refresh 检查：在当前边界到达 deadline 时，先设置下个 deadline 为当前 cycle 加 896，再增加 14 master cycles，随后发生该操作的 IO 和固定架构成本。原版 PSG 路径直接调用 `psg_write(m68k.cycles, data)`，本服务不增加 YM／ZRAM 的 7 master cycles 访问延迟，也不请求 bus 或查询 YM busy。

默认操作预算为 4,096，上限为 65,536。raw store 使用 1 次固定操作；single、pitch-pair、mute-all 分别使用 2、3、6 次。未知 kind、无效 payload、缺少 owner、超过上限或明显溢出的起始 cycle 均在外部 IO 前拒绝。错误返回零值 clock；预算或外部错误可能出现在此前已有写入之后。调用者必须使用 detached owner，失败时丢弃 transaction，成功后再提交。跨帧 flush 仍由调用者显式处理。

## 原生证据与先行测试

[先行失败场景](SCORE_CLOCK_PSG_FAILURES.md) 和 [test-first.json](../../../games/yuyuhakusho/verification/score-clock-psg/test-first.json) 先于实现。实际 red 是缺少 `PSGKind`、`StorePSG`、`ServePSG` 等 API 的编译失败。实现后没有修改冻结测试或 native 期望。

native oracle 从既有 quiet world 检查点开始，每次恢复后明确执行 PSG reset，构造 private origin、refresh deadline、canonical 字节和返回边界。原 core 自己执行未修改的原指令。138 个配置覆盖 raw store 48 例、single 48 例、pitch-pair 36 例与 mute-all 6 例，包括两个 single 来源、三个 tone channel、分频边界及六种起点／deadline 相位。

每例分别执行 recorder 关闭／开启的对照，两份 core 共 552 次原操作执行；每份 core 的完整 serialized state、RAM、refresh、PSG context 与逐步执行边界均不受 recorder 影响。两份 core 的有序 PSG 写入、最终时钟和 PSG context 也完全相同。原 PC、opcode 和逐指令成本只保留在研究产物；portable fixture 仅保存请求、期望写点、结束时钟和芯片语义状态。

Go 与 Wasm 比较全部 192 次有序写入及结束时钟，并比较 native 的 latch、tone divider、level。另以相同 native 写点直接调用真实 `chips.Interactive`，核对完整 owner snapshot，确认服务没有额外写入、隐式 flush 或第二份状态。失败路径覆盖无效输入、有限预算、外部错误传播、倒退／未 flush 输入和后续 refresh 溢出。

Wasm runner 初次设为 `GOMAXPROCS=2` 时，在测试执行前遇到 `newosproc: not implemented`。原始环境失败日志已保留；runner 现明确对 Wasm 使用 `GOMAXPROCS=1`，测试和实现均未为此修改。

验收证据由 [validation.json](../../../games/yuyuhakusho/verification/score-clock-psg/validation.json) 汇总。portable fixture 的 SHA-256 为 `2ebb7999315f787af32118c5f6b25ffeb38a6973a7db4a40e714eed3a9ffff56`；包含原 PC／步骤的完整 native 产物 SHA-256 为 `546b19c0f95ce6c3bde68c7981b07cc66b4a839cddddd274b3fe9d0a5673c7fd`。

## 复跑与证明范围

在本工作树执行以下命令。Go runner 为全部 Go 调用设置工作区内的 telemetry、cache、module cache、临时目录；不运行 `go telemetry`。

```sh
PYTHONDONTWRITEBYTECODE=1 nice -n 10 python3 scripts/yuyuhakusho/score-clock-psg-verify.py \
  --mode native --out .local/md-yuyuhakusho/score-clock-psg-go-review
PYTHONDONTWRITEBYTECODE=1 nice -n 10 python3 scripts/yuyuhakusho/score-clock-psg-verify.py \
  --mode wasm --out .local/md-yuyuhakusho/score-clock-psg-go-review
PYTHONDONTWRITEBYTECODE=1 nice -n 10 python3 scripts/yuyuhakusho/score-clock-psg-verify.py \
  --mode vet --out .local/md-yuyuhakusho/score-clock-psg-go-review
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/score-clock-psg-cost-source.py \
  --out .local/md-yuyuhakusho/score-clock-psg-cost-review.json
PYTHONDONTWRITEBYTECODE=1 nice -n 10 python3 scripts/yuyuhakusho/score-clock-psg-capture.py \
  --out .local/md-yuyuhakusho/score-clock-psg-native-review
```

native 重采使用本工作树已有的固定 ROM 与两份 core。原生采集保持单进程，并与其他采集／浏览器性能窗口协调。重建 portable fixture 时，为 capture 命令显式增加 `--fixture games/yuyuhakusho/audio/scoreclock/testdata/psg-native-services.json.gz`。

本范围不修改旧 `ServeYM`、score、chips、phaseclock、engine 或总索引。PSG reset fixture 不是自然 driver 调用路径；native serialized state 与 PSG context 的相同也不单独证明输出缓冲区／PCM 相同。caller 准备成本、完整 driver tick 赋时、自然调度、产品声音播放、实体硬件和发布仍须各自验证。
