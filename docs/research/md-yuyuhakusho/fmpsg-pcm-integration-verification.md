# 独立 PCM Device 与完整芯片组合验收

完整 23 组 PCM 命令场景已分别通过原端口重放和独立 Go Device 组合检查。两层各覆盖 1,274 帧、937,609 个立体声 PCM16 采样及完整 typed YM／PSG／FIR 状态；独立 Device 还逐项产生相同的 519,256 个 physical-port／bank／reset ledger 事件、反馈和总线事务。Native、race、Wasm 和 vet 通过，现有生产源码未修改。

验证索引为 [fmpsg-pcm-integration-validation.json](../../../games/yuyuhakusho/verification/fmpsg-pcm-integration/fmpsg-pcm-integration-validation.json)。本项新增数值组合证明，不代表世界／IRQ 赋时、活动音序混合、平台播放或物理扬声器验收完成。

## 分层输入与结论

| 层 | 输入 | 已验证 |
| --- | --- | --- |
| 新 native oracle | 原实际 address／data／reset 端口序列；直接读取的完整 quiescent 初态 | 原版和 hooked core 的完整初态、逐帧 YM／PSG／FIR／PCM 相同；原 CPU 控制中已记录 PCM、DSP scalar 和 latch 也相同 |
| Go A：给定原端口 | 同一原端口记录，保持原数组顺序和 frame group | 独立完整芯片输出逐采样／逐字段等于新 native oracle；此层不给 Device 自主生成事件记功 |
| Go B：独立 Device | immutable payload／descriptor、typed accepted命令／hold／DMA、完整已声明芯片初态 | Device 自主推进，生成事件只作事后原记录断言，真正送入 DSP 的是 `generated.Ledger`；完整输出等于同一 native oracle |

Go B 不把原未来事件时刻传入 Device。它每帧只提供属于该 frame 的 typed 外部输入和 DMA 区间；资源按内容 ID 读取，独立语义状态机决定 prefetch、decode、bank、address、data 和 reset 时刻。测试另外序列化恢复 Device，并在本帧输入完成、尚未 flush 时恢复完整 Interactive 快照，随后继续原输出轨迹。

23 组包含 single／dual、追加／替换、音乐 DAC、pause、restore-pan、DMA、7F 私有诊断、F0 合法 remap、hard reset、DAC 操作中 reset，以及 address 已写而 data 未写的 reset。各层原生／race／Wasm 都保留 23 个独立场景结果，不能用只检查生成事件或只检查 address 来替代完整输出证明。

## 完整初态与原生恢复

初态来自同一 `quiet_checkpoint`，随后执行与原命令捕获相同的 `initial(ref)`。在这一步前后被动读取完整 YM operator／channel／timer／LFO、PSG divider／noise／pending delta、FM 输出、integrator 和当前 FIR，并确认 PCM 初始化没有改动芯片初态。Go 直接使用该完整 typed 初态，不由旧有限 `InitialDSP` 拼造，也不替换成冷 reset。

原 native savestate 省略 FM busy、完整 sample progress 和 FM buffer pointer。新观察器在 quiescent 点读出 sidecar 并核对每次恢复。此次两个 core 的实际 sidecar 均为 `start=0,count=0,busy=0,ptrOffset=0`，23 场景全部已自然恢复一致，没有执行补写。完整原生初态的 SHA 在两个 core 及全部场景相同。

旧 CPU fixture 仅提供有限 DSP scalar 和每帧 address latch；完整 YM／PSG／FIR 字段由此次 native 端口重放额外采集，不能把未记录的 CPU 内部字段补称为直接 CPU 对照。旧 `DSP.InputAhead` 还是 exporter 的 DAC-only 输入 cursor，不是完整 physical-port frontier；新的完整 frontier 由真实端口输入和 native frame 上下文独立记录，不拿该有限字段冒充完整状态。

## 原调用序与真实失败

地址端口使用 v3 的真实 port0／2、真实 cycle；data 使用真实 port1／3，不临时补地址。metadata／bank／dummy 只承载已有调度信息，已计入后续时刻的 wait／DMA effect 不再额外添加。每个返回 frame group 只做一次原参考 flush，不按 `Cycle/F` 重分帧；raw lookahead、reset 回退和全部首尾数据均保留。

reset 只有在原 cycle 早于当前 frontier、且前一个实际 chip event 是同 frontier 的完成 DAC data 时，才进入已冻结的 `ResetYMAfterDAC`；其他 reset 使用普通方法。没有用 Device.Time、address时刻或任意 YM／PSG 访问拼造 witness。

[先行失败场景](fmpsg-pcm-integration-failures.md)和 E2E 在组合执行前保存。两个明确的错误桥接假设实际失败：

- data-only 模式丢真实 address，并在 data cycle 补地址；`single-reset-between-address-data` 的 frame0 完整状态为实际 `Address=43`、原 `Address=42`，即 2B／2A。
- 按 raw cycle 排序；`single-reset-crossing-chip-tick` 在 `frame=8`、交织 `sample=28` 首差，对应 `first_byte=56`。索引从 0 开始。

这两个 red 是负向错误假设检查，不声称现有生产 API 原本有 bug。正确模式第一轮即通过，无需修改生产或放宽断言。首轮之后只补充了失败 frame 的日志位置；旧测试字节和旧红／绿日志保留，比较逻辑与输入未改。原 owner 的 data-only latch red、真正跨 chip tick 的排序 red、以及此前 20 组无差异误标订正也独立保留。

## 复跑与范围

```sh
TEST_TELEMETRY_DIR="$PWD/.local/go-telemetry" \
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -json -count=1 -run '^TestPCMFullChip' ./games/yuyuhakusho/audio/chips
```

Wasm 的最小环境和精确命令由验证索引保存。新 native 捕获可在与其他分工协调的窗口，以低优先级单进程输出到新目录：

```sh
nice -n 15 env PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/fmpsg-pcm-integration-capture.py \
  --out .local/md-yuyuhakusho/fmpsg-pcm-integration-review
```

Go A／B 重放无需 ROM 或 native core；重新取得 native oracle 仍需固定原研究资源。初态是明确给定的完整芯片状态，本项没有独立重建整个原启动流程。Native 配置仍为已冻结的离散 YM／集成 PSG、HQ、44.1 kHz、filter0；本项不扩展为所有硬件或媒体设备的声音等价承诺。
