# 帧内交互 DSP 与原生反馈循环

`audio/chips` 新增即时 YM／PSG 写入和 status 读回，支持时基调用者依据刚读到的 busy bit 继续轮询或写入。5 组原生场景、1,109 次交互、13 帧、9,563 个立体声 PCM16 采样，以及每步 typed 芯片和当前 FIR 状态全部一致。Native race／Wasm 各 17 项顶层检查、62 个子场景通过，包含全部旧 raw YM、混合 PCM、PSG 和 DAC oracle；vet 通过。

本项给定每次端口访问的 cycle，只证明即时 DSP 反馈、状态与帧 flush。原 driver 的指令语义成本、独立 IRQ／总线时序及世界声音赋时仍由相应 owner 恢复，不把私有轮询步长当成原游戏成本。接口见 [INTERACTIVE.md](../../../games/yuyuhakusho/audio/chips/INTERACTIVE.md)，完整索引见 [validation.json](../../../games/yuyuhakusho/verification/fmpsg-interactive/validation.json)。

## 同一执行器与唯一 owner

新交互实例直接保存 YM、PSG、busy、raw cursor、已生成 FM 采样数、未读 integrator 和一个当前 FIR buffer。FIR buffer 取代旧 `Tail`，不保存第二份尾表、旧初态或已消费事件。下个 FM tick、采样数和 resampling phase 临时推导。每步快照拥有足够的当前数据，不借助原 context、host 指针或隐藏输入历史恢复。

旧 `Stream.FrameBoundary` 只作最小转接：把既有 `Pending` 依序交给相同事务，再执行同一个 flush。原公开签名和 `Frame.Reads` 规则不变。旧来源冻结 manifest、旧日志和旧 fixture 保留原样；旧 `stream.go` 的完整字节另保存于 `stream-before-interactive.go.txt`，其 SHA 与原冻结记录逐项一致。新验证绑定当前实现，不能把旧 manifest 的旧 source SHA 当作当前源码验证。

每次操作先校验端口、时间、容量和 overflow，在临时芯片候选上生成边沿，再验证所有 FIR 索引，全部成功后才提交。不为每次 busy poll 复制整块 FIR。帧尾使用一次有界临时副本保证整个 flush 失败时也不提交部分 PCM 或状态。

## 原生证据与 FIR 投影

原生 fixture 每次使用 native audio-only reset 明确初始化 YM／PSG／blip，不跑 gameplay CPU 帧，不使用 native 半帧 savestate。原版和 hooked core 对同一输入逐操作、逐帧相同。所有非音频 guest RAM 和视频保持不变。

原 core 把本帧 FM 输出暂存在 raw buffer，到帧尾才注入 FIR；Go 直接把已生成的帧内 FM delta 放进当前 FIR。为比较中间表示，观察器先复制原 blip 到独立 native blip，再将原 `fm_buffer` 中帧内采样通过未修改的 `blip_add_delta` API 逐笔注入该副本，得到等价的当前 FIR 投影。超前 FM raw 采样仍排除在该投影之外；PSG 早已提交的 ahead delta 保留。

每个 core 有 1,127 次无扰动检查，均确认投影没有改变原 YM bytes、PSG bytes、完整 blip buffer／offset／integrator 或非音频 guest 状态。投影只是不同流水表示之间的规范化，不宣称它是某个原字段的直接读取值。新 Go 测试每次操作后都完整 snapshot／restore，并与该投影及 native scalar 状态逐项比较，最终 PCM 又独立对照真实 native `audio_update` 输出。

原声系统的 savestate 并不完整保存半帧 `fm_cycles_busy`、`fm_ptr` 和 raw buffer 进度。本项没有利用 native save／restore 去冒称这些状态等价，也没有修改通用 `reference.py`。Go 半帧恢复的证据来自明确初态的连续原生序列和自己的完整 typed 状态。

## 覆盖与先行失败

| 场景 | 验证 |
| --- | --- |
| `busy-grid-boundaries` | 42 master grid，busy 边界前／等于／后，port0与其他 status 端口 |
| `ports-psg-same-cycle` | 共享地址／bank latch、同 cycle 保序、PSG clock边界与 FM混合 |
| `explicit-frame-overrun` | F+195／F+1008 raw 输入、显式 flush、busy carry、FM副作用与 PSG tail |
| `adaptive-service` | 真正用每次返回 busy 控制下一次原生 poll／写入；Go 走相同反馈程序并生成相同访问序列 |
| `status-timers-mixed` | Timer A／B／CSM status 与 PSG混合，连续四帧 |

[FMPSG_INTERACTIVE_FAILURES.md](FMPSG_INTERACTIVE_FAILURES.md)、完整 Go E2E 和 fixture producer 先于新实现。缺少 `InteractiveState`／`NewInteractive`／`RestoreInteractive` 的真实 red 保留。所有新测试在生产前冻结，原生 fixture 在实现前取得；没有放宽旧 PCM／状态断言。

原子失败检查另覆盖非法端口、回退／超前时间、origin overflow、操作容量、伪造半帧状态、FIR slice alias、半帧误导出旧 State，以及带未消费 Pending 的模式切换。达到操作上限不会自动 flush；显式 flush 后恢复容量。

## 复跑

```sh
TEST_TELEMETRY_DIR="$PWD/.local/go-telemetry" \
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -json -count=1 ./games/yuyuhakusho/audio/chips/... ./games/yuyuhakusho/audio/dsp
```

Wasm 的最小环境、精确命令与当前来源 SHA 由新索引记录。重新采集必须输出到本 worktree 的新目录，保留冻结证据；原捕获按低优先级单进程执行：

```sh
nice -n 15 env PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/fmpsg_capture_interactive.py \
  --out .local/md-yuyuhakusho/fmpsg-interactive-review
```

无需读取 ROM／core 即可重放已保存 Go oracle。原始 native fixture 重新取得时必须同时通过原版和 hooked core 及无扰动投影检查，不能用 Go 输出覆盖 original。
