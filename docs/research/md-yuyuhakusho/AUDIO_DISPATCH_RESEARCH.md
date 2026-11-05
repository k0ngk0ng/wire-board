# PCM 分派选择与控制命令边界

已恢复纯 Go `SelectPCMCommand`，用于原 `37d66..3807c` 的 PCM 接收、拒绝、单 DAC／双槽选择、自动偏好和 duck 请求判断。四项测试在原生竞态和 Node／Wasm 通过。它返回分派意图，不执行 Z80、合成或播放，也没有接入世界时钟。

机器索引为 [audio-dispatch/validation.json](../../../games/yuyuhakusho/verification/audio-dispatch/validation.json)。先行范围见 [AUDIO_DISPATCH_FAILURES.md](AUDIO_DISPATCH_FAILURES.md)。静态证据沿用 [audio-dispatch.txt](static/audio-dispatch.txt)、[audio-control.txt](static/audio-control.txt)、[audio-loop-sequencer.txt](static/audio-loop-sequencer.txt) 和 [audio-z80.txt](static/audio-z80.txt)；它们来自 direct Ghidra，不是 REA 原生 raw-68000 反编译结果。

## 语义视图与唯一 owner

| 字段 | 原来源 | 本阶段用途 |
| --- | --- | --- |
| `PCMArbitrationCommand.Flags` | descriptor byte 5 | bit 0 选择单／双模式；高两位为仲裁等级；bit 5 参与 duck 请求 |
| `RoutingOrPan` | 同一 descriptor byte 6 | 单模式为 YM2612 B6 pan；双模式按高两位选 A／B／自动；保留同一 byte owner |
| `ModeFlags` | 原分派器实际读取的 `1ffc` | 当前模式及优先级；使用 `37da4/37e9e` 的真实寄存器值 |
| `SlotFlags` | `1f87/1fa7` | 各槽 flags 的高两位参与比较 |
| `SlotBusy` | `1f86/1fa6` | 只有 0 表示可直接选用；待装入 `80`、活动 `40` 和其他非零值均不是空槽 |
| `ChoiceBits` | `F82C` | 自动选择偏好；只修改 bit 0，保留其余位 |
| `DuckMode` | `F82B` | 有条件请求设为 1；不是已完成的音量渐变 |

这些输入都是临时值视图，不另建持久 Z80 RAM 镜像。`PCMArbitrationDecision` 只有 `Destination`、`ChoiceBits`、`StartDual` 和 `DuckMode`，不保存输入队列、暂停、寄存器或固件。

单模式按当前模式等级或两个槽等级比较，较低请求拒绝。双模式从单模式切入时也先检查当前等级。定向 A／B 始终检查目标槽等级，不能因为该槽 busy=0 就绕过等级比较；自动模式则先使用空 A，再使用空 B，再按偏好及等级选择。相等等级允许覆盖。A 接收后置偏好 bit 0，B 接收后清该 bit；拒绝及单模式保持原 byte。

已接收且新 flags bit 5=1、旧 mode bit 5=0、原 duck mode 不为 2 时才请求 duck mode=1。后续 `38c2e` 的实际渐变、恢复和活动音乐声部变化不在此 helper 中。

## 原调用与原版对照

私有原调用从已验证静音检查点恢复，并总是补齐 `refresh_cycles`。在真实 `37d66` 入口执行原 ROM 指令，输入当前播放 flags／busy／选择 byte；不修改原指令或 descriptor。采样保留真实读寄存器、接收分支、双模式启动分支和返回边界。

- 13,036 次私有原调用：492 次覆盖 `06..7f` 的全部 122 个 descriptor 及额外 `F0` descriptor，各 4 种模式；12,288 次边界；8 条单初态流合计 256 次连续决策。
- 边界覆盖单／双模式、当前等级、两个槽的不同等级、空／`40`／`80`／`FF` busy、自动偏好高位与 duck 状态。覆盖的是实际 descriptor 的 flags／routing 集合，不宣称穷举所有可能 descriptor byte。
- 每次比较完整 64 KiB M68K RAM 差异。除 `F82B/F82C` 和实际调用栈 `FEE0..<FF00` 外，未知写入为 0。自主 Z80 内存和硬件合成不是本 helper 重建的对象。
- 23 组单音、成对、优先级、模式切换和控制输入，各 190 视频步骤。原内核和观察内核共 8,740 步的完整 RAM、视频、PCM 均一致；33 个实际队列分派的 PCM 决策另与 Go 比较。

全部成对输入均为清除世界生产者后的明确私有队列输入，不冒充正常玩法可达。真实正常世界的音频时基和回调内消费证据由 [AUDIO_TIMING.md](AUDIO_TIMING.md) 单独维护。

## 控制与别名的实证边界

| 输入 | 已观察事实 | 未由本阶段证明 |
| --- | --- | --- |
| `02` | 原 `38808` 针对 `FA20..FB10` 主 FM／PSG 效果行；正在播放的两个 PCM 槽在返回与下一步仍为 `40` | 完整 live FM／PSG 声部恢复和抢占算法 |
| `03` | 原 `388ac` 处理 `FB40/FB70` 副效果行；同样没有停止本批正在播放的两个 PCM 槽 | 同上 |
| `04` | 清理 M68K 驱动区与 key-off／PSG 静音；本批两个 PCM 槽仍继续，直至原生结束 | 不能映射为「立即停止所有 PCM 媒体」 |
| `00` | 静态为上传 Z80 固件并复位；明确私有 `SelectedSound=00` 输入使两个 PCM 槽清为 0 | `RequestSound(00)` 并不能提交复位，因为 queue 中 0 是空值 |
| `F0` | descriptor index 为 `0x7a`（122），位于 `06..7f` 的普通表项之后；实际接受 B 槽 | 它不是编号 `7a` 的 WAV，也不在已验 134 个有限媒体目录中 |
| `01/05` | 原设置 fade 计数与间隔，属于控制，不是媒体 | 完整淡出、duck、声部恢复与混音时基 |

首轮采样假定 queue00 可以复位，实际只收到两次而非三次分派，已保留真实失败和旧 producer。修复后只把复位 case 标为私有选中编号输入，未改变原程序。另两个 `a0→02/03` case 中，`a0` 在第 8 步停止命令到达前已经自然结束，因此不能用这两个 case 宣称已验证 live FM／PSG 停止；上表的行组范围来自静态证据。

## 两个单音 WAV 不能直接代替原混音

保留三个可独立读取的原 PCM WAV：A 在第 0 步提交 `1f`，B 单独在第 8 步提交 `3f`，以及同一静音检查点和时基上的 A+B 配对输入。三者各 139,831 个立体声采样帧。第 8 步边界为采样帧 5,888；此前配对输出与 A 完全相同，B 全为精确零。

在这一确定输入对齐下，未裁切、未重新对齐的 16 位饱和线性和有 161,234 个样本值不同，最大绝对误差 23,063，且没有任何相加饱和。进一步用整数正规方程求同一对齐下任意固定双增益的唯一最小二乘解，仍有 173,016 个非零有理残差样本，因此两条固定增益也不能精确重建这一输入下的原配对 PCM。

没有做任意波形移位搜索，也没有据此断言差异只有一个原因。单独 B 从空模式启动双槽固件，而配对 B 加入已经运行的双槽模式；原启动／解码／混合／滤波状态与时间均可能相关。当前反例否定直接把隔离文件叠放当作原混音，未证明任意状态相关 DSP 方案不可行。平台有限声道可并发播放，不代表本节原 PCM 已重建。

## 复跑

不依赖模拟器的 Go 验证：

```sh
TEST_TELEMETRY_DIR="$PWD/.local/go-telemetry" \
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -json -count=1 ./games/yuyuhakusho/engine -run '^TestPCMArbitration'
```

Wasm 的最小环境与完整命令保存在 `verification/audio-dispatch/wasm-command.json`。无需原工具即可复验保留的 PCM 反例：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/analyze_pcm_mix_counterexample.py \
  --out .local/md-yuyuhakusho/pcm-mix-review.json
```

增加 `--expect-linear` 应返回非零，保留真实反例。重新采集完整原输入与私有分派边界：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_audio_dispatch.py \
  --out .local/md-yuyuhakusho/audio-dispatch-review
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_pcm_arbitration.py \
  --out .local/md-yuyuhakusho/pcm-arbitration-review
```

两份 original／hook capture 的完整清单保留在 `.local/md-yuyuhakusho/audio-dispatch-study/`。生产代码只新增纯选择 helper；未接入世界、声音队列消费位置、Z80 确认、PCM 解码、音乐声部、平台媒体或发布流程。

本分工清理 5 个私有重复／空临时目录，共 330,900 bytes。规范原控制与观察数据、私有边界、PCM 反例、原始失败和最终日志保留。完整私有采集及两组导出夹具的重复 gzip 逐 byte 相同，路径字段不同的 manifest 不声称逐 byte 相同。
