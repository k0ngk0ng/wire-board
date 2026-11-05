# 独立 PCM 节拍、预取与 DAC DSP 链路

独立 Go `audio/pcmplayer` 已从已初始化的 typed 播放器状态和 DPCM 资源，自主生成连续 DAC 与总线读取事件。12 组原生输入的 70,856 个事件逐项相同，包含 master 时刻、输出 byte、资源读取位置、dummy 读和停止顺序。接入已有 DAC DSP 后，302 个参考帧、222,258 个立体声 PCM 帧及每帧完整 typed DSP 状态逐项相同。

这项证明限定为已初始化播放器、无外部暂停的固定参考 profile。世界规则、68k、命令发布／抢占、DMA 与 IRQ 未在本次独立调度中执行。机器索引为 [audio-pcm-player/validation.json](../../../games/yuyuhakusho/verification/audio-pcm-player/validation.json)，API 为 [pcmplayer/README.md](../../../games/yuyuhakusho/audio/pcmplayer/README.md)，先行失败范围为 [AUDIO_PCM_PLAYER_FAILURES.md](AUDIO_PCM_PLAYER_FAILURES.md)。

## 原生 oracle 与独立预测的边界

原生采样恢复既有静音检查点，明确设置 Z80 的播放器入口和对应数据，随后只调用固定原固件的 `z80_run` 与原音频 flush。世界／68k 不推进，DMA 及外部 bus hold 不产生。双槽从描述已转交、空缓冲开始；单槽从 128-byte 预装完成开始。研究进程执行原程序，生产 Go 只执行 typed 语义和静态成本求和。

原内核与观察内核每一步的完整 Z80 寄存器、RAM、PCM、视频与世界 RAM 等值。观察桥复用 `pcm_model_observer.c` 的函数指针被动委托，不改变 handler 输入、返回值或 cycle。结束时严格确认 wrapper 未被替换，事件溢出为 0。原 PC 只在研究分类器中识别观察事件来源；portable 夹具没有 PC、指令或 ROM 地址。

第一批 7 组及扩展 5 组用于事件与播放器状态证明。该批私有入口没有启用静音检查点中的 DAC，PCM 不能作为可听链路证明。之后单独采集 12 组 `audible` 输入，明确在 0 周期写入 DAC128、enable80、panC0，保存真实芯片／resampler 初态。它们包含 437,222 个非零原生采样值，旧记录没有被覆盖。

| 场景 | typed 输入 | 覆盖 |
| --- | --- | --- |
| `dual-a` / `dual-b` | 原 06／7A 的单活动槽 | 槽方向、空槽中性值、完整尾部 |
| `dual-pair` | 原 1F／3F 同时活动 | 双槽限幅、独立源结束、连续长流 |
| `dual-odd-tail` | 06 的 259 字节、7A 的 133 字节 | 四字节预取的非整轮尾部 |
| `dual-min69` | 06 的前 69 字节 | 预取超过 64 后可开始解码的最短有限合同 |
| `single-native` | 原 73 的完整 7,788 字节，rate3 | 128 字节初态、预取与 dummy 读取交替 |
| `single-tail129` / `single-tail257` | 同源前缀，rate3 | 双字节预取下溢、尾部目标位置 |
| `single-rate000/001/020/255` | 同源前 129 字节 | rate0 的 256 次循环、最小及较大速度字段 |

私有截短长度是明确的测试输入，未宣称这些短描述能由正常游戏菜单产生。7 个初始原生组的第二次完整采集逐 byte 相同。12 个 audible 组再次采集后，完整 JSON 值、全部原事件、PCM 与 typed DSP 状态相同；新采样器将 `initial_dsp` 键追加到对象末尾，序列化顺序不同，故仅该次报告使用完整 JSON 等值和规范排序 SHA，不声称压缩文件 byte 相同。

## 自主节拍与状态

播放器只保存两个互斥的 typed 状态变体。双槽保留 `Consumed/Fetched/Accumulator/Enabled/Active` 和下一次预取的槽；单槽保留 `Consumed/Fetched/Accumulator/DummyCursor`。缓冲地址低字节、源已产生结束位置、单槽尾部阶段均由当前状态派生。已知不可变数据可以直接按消费位置解码，不必保留一份原环形 RAM。

原双槽每轮解码两槽各一字节，先输出高 nibble，轮流处理一个槽的四字节预取，再输出低 nibble。无活动槽贡献 128。源读取末尾可多读最多 3 字节；这些 byte 只占总线时间，不进入输出。源预取与解码活动分开，因此 `Fetched` 达到末尾不等于立即停止发声。

单槽在 128 字节预装后，以缓冲差决定预取两个字节或执行 dummy bank 读取；源全部读完后仍排空已有缓冲。rate 延迟为 `13 × (rate == 0 ? 256 : rate) - 5` 个 Z80 T-states。dummy 游标有后续总线行为，独立保存；它的返回 byte 不进入解码器。

单槽有限 profile 明确使用 bank 窗口起点的 dummy 游标，长度限制为 129～7,788 字节。复核 API 时发现初版允许更长资源，先新增负向测试取得真实失败，再收紧合同。该拒绝没有修改 7 个冻结原生场景或放宽对照。双槽长度合同为 69～32,768 字节。

## 静态语义成本

所有时刻由语义分支和静态求和产生。每个 Z80 T-state 为 15 master cycles。没有把原未来时刻、寄存器或 PC 时间表输入生产。

| 语义块 | T-states |
| --- | ---: |
| 双槽轮前固定初始化 | 45 |
| 预取 A 的轮：A 启用／关闭解码 | 137 / 142 |
| 预取 A 的轮：B 启用／关闭解码 | 168 / 173 |
| 预取 B 的轮：A 启用／关闭解码 | 171 / 176 |
| 预取 B 的轮：B 启用／关闭解码 | 134 / 139 |
| 双槽高／低 DAC 写入块 | 52 |
| 双槽读完低半段后的固定阶段 | 38 |
| bank source 读取的原 LDI 基础成本 | 16 |
| 固定参考 bank 等待 | 3，即 45 master cycles |
| 单槽从字节阶段开始到高 DAC | 110 |
| 结束入口到恢复 pan / 关闭 DAC / 返回 idle | 106 / 183 / 232 |

双槽预取还根据源剩余、缓冲差、活动状态和末尾下溢走不同语义路径。单槽有 rate 延迟、两字节预取和 dummy 读的独立分支。完整逐事件相等来自这些条件，未用波形拟合常数。

静态来源为现有 direct Ghidra 的 [audio-z80.txt](static/audio-z80.txt)，固件范围 `393FE..39FFD`，SHA `42ed1d2c4133d88d36c4024ff535a710f064dcb5d791b892eb3a07f6ded6ef27`。固定 core 的 cycle 单位和 bank 等待来自其 Z80 与 `memz80.c`。这是固定参考核的经验模型；真实硬件 pin-edge 没有在本组测量。

## 按操作开始时刻分配参考 frame

每个事件同时给出 `StartCycle` 与实际 handler 的 `Cycle`。原 `z80_run` 在内存操作开始前判断本轮预算；该操作开始于 frame 内，即使 handler 写入溢出 frame 末，仍属于这次 native 调用。

链路测试只把自主播放器的 `DAC/Pan/Enable` 交给 `audio/dsp`。原 oracle 的时刻不用于驱动 Go。分帧按 `StartCycle / 896040`；DSP 仍收到未经 clamp 的原预测写时刻，处理其 lookahead 与 frame flush。

错误地按 `Cycle / 896040` 分帧有真实反例：`single-native` 第 8 帧末原 DACValue 为 137、InputAhead 为 165；错误策略留下 DACValue145、InputAhead0。此时该帧已 flush 的 PCM 仍可能相同，所以验收同时比较完整 typed DSP 状态。错误模式通过环境变量复现，正常模式保持全部 PCM 与 state 零差异。

## 验收与复跑

原生竞态和 Node／Wasm 各 6 项顶层检查、37 个子场景通过，`go vet` 通过。原连续播放器事件为 70,856 笔；自主播放器到 DSP 为 302 个参考帧、222,258 个立体声 PCM 帧。自己的字节边界保存恢复不改变结果，DSP 在不同帧保存恢复也不改变输出。

Go 测试不需要 ROM 或原模拟器：

```sh
TEST_TELEMETRY_DIR="$PWD/.local/go-telemetry" \
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -json -count=1 ./games/yuyuhakusho/audio/pcmplayer
```

重复原生采样只写 caller 指定的工作区目录：

```sh
PYTHONDONTWRITEBYTECODE=1 TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
python3 scripts/yuyuhakusho/pcm-player-capture.py \
  --out .local/md-yuyuhakusho/pcm-player-native-review --audible
```

Wasm 精确环境与命令见 `verification/audio-pcm-player/wasm-final-command.json`。错误分帧策略应返回非零：

```sh
YUYU_PCM_PLAYER_VERIFY_FRAME_END_ADMISSION=1 \
TEST_TELEMETRY_DIR="$PWD/.local/go-telemetry" \
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -count=1 -run TestAutonomousPlayerThroughDACDSPAgainstOriginalPCM \
  ./games/yuyuhakusho/audio/pcmplayer
```

后续需要恢复从空闲到本初态的命令事务、运行中替换／抢占与暂停、外部 68k bus hold、DMA、世界 producer 和 IRQ 的协作时序。`Next` 当前在一个字节内原子推进，外部事件若落在该阶段中间，未来须细化可中断的 typed 阶段；不能在 `Next` 之外统一平移整轮来替代。本组没有修改 `audio/pcm`、`audio/dsp`、engine scheduler、平台或声音发布，也没有接入实际对局声音。
