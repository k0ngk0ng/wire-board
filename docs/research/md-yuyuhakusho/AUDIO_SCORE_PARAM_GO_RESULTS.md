# 独立 Go 音序参数

`audio/score/param` 已实现 typed FM 音色／音量、pitch 与 PSG volume envelope、E4 声像推进。原生竞态与 Node／Wasm 的 4 项检查全部通过，`go vet` 通过。生产文件不读取 ROM、原指令、地址、PC、RAM 镜像或 oracle；测试夹具仅由测试文件嵌入。

机器索引见 [audio-score-param/validation.json](../../../games/yuyuhakusho/verification/audio-score-param/validation.json)，先行范围见 [失败场景](AUDIO_SCORE_PARAM_GO_FAILURES.md)。这项结果是参数层证明，完整 driver 的 gate、tie、modulation、claim／restore、有序声音意图和 DSP 仍由父分支连续验收。

## 正式接口

包路径为 `github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score/param`。参数结构的 JSON tags 与已提炼 catalog 一致；`FMPatch.Operators` 固定为 4 个按硬件 register slot 排列的 operator。反序列化拒绝多余／不足的 operator，不让标准库静默丢弃第五项。

| API | 输出与归属 |
| --- | --- |
| `FMPatch.Validate()` | 检查字段范围及 algorithm 对应的 carrier；不虚构 SSG 参数 |
| `PatchWrites(patch, base)` | 返回 25 条 channel-independent、chip-effective `FMRegisterWrite`；顺序为 B0、五组 operator 参数、TL |
| `FMLevelUpdates(patch, base, initial)` | 返回按原 operator 写入顺序排列的 `FMLevelUpdate`；initial 写全部 operator，ordinary 保留 signed／carry 跳写 |
| `PitchEnvelope.Next(cursor)` | 返回当前 `PitchTick` 和下个 typed `EnvelopeCursor`；结束／越界返回错误并保持输入 cursor |
| `VolumeEnvelope.Next(cursor)` | 同样使用 typed 前缀／循环位置；保留 hold 的交替无写与重复 keyoff |
| `PitchWord(tick, scale, base, detune, modulation)` | 返回低 16 位计算结果和 byte wrap 后的 signed scale；keyoff bypass 不加 base／detune／modulation |
| `PSGAttenuation(tick, base)` | 先 byte wrap，再限制为 0～15；无写 tick 明确返回 `write=false` |
| `InitialPanState(definition)` | 返回定义的初始 step／elapsed；仅用于确实需要初始化／重置的路径 |
| `AdvancePan(definition, patterns, state, noteBoundary, keyOff)` | 返回 `PanTick{State, Pan}`；`Pan=nil` 表示无写，`Stereo.Bits()` 只给高两位声像，低 LFO 位仍归 caller |

以上运算函数除 `InitialPanState` 外均返回错误。定义与 tick 中的可选值以指针表达 JSON 的 `null`，属于不可变参数视图；调用者不应修改其指向内容。运行状态只保存 `EnvelopeCursor{Cycle,Index}` 和 `PanState{Step,Elapsed}`，不另存等价累计 tick 或原游标。

## 原版预期和覆盖

失败清单、Go 检查和实际 fixture 先于生产 helper。首次运行保存了 `undefined: FMPatch` 等缺失实现的真实 red。之后仅对测试作 `gofmt` 排版，断言和期望不变；实现完成后原检查全部通过。

| 检查 | 原版／独立来源 | 数量 |
| --- | --- | ---: |
| FM patch 请求 | 已保存的实际原版读 trace，保留顺序后仅投影明确的硬件忽略位 | 2,525 次 load，63,125 条请求 |
| Initial／ordinary 音量边界 | 固定源 patch 的完整 TL byte 与原加法／分支语义；不是调用 Go helper 生成期望 | 100 个 patch × 256 个 base level，共 25,600 组，每组比较两条路径 |
| Pitch envelope 连续推进 | 冻结的独立原表解释器，从零 cursor 连续推进 | 8 条 envelope，共 4,270 个 tick |
| PSG volume envelope 连续推进 | 同一独立源解释器，保留 80／81／82／83 行为 | 11 条 envelope，共 22,528 个 tick |
| E4 声像推进 | 原表读取与原指令效果；含 27 组实际参数、全部 5 个 mode 及 period 0／1／3／255 | 52,200 次调用 |
| 非法参数与算术 | 明确边界：数量、carrier、unknown mode／pattern、cursor、byte／word wrap、terminal keyoff | 原生与 Wasm 相同 4 项顶层检查中的边界部分 |

FM 请求投影逐寄存器声明：`30:7f`、`40:7f`、`50:df`、`60:9f`、`70:1f`、`80:ff`、`b0:3f`。请求条数和顺序没有筛掉；这里不声称能从 typed 字段重建所有原 8 位请求。TL bit 7 虽被 YM 忽略，却参与 driver carry，已由 `volume_carry_bias` 独立保留。

实际 pitch envelope 在这 23 个定义中没有读取；其 Go 连续结果属于静态／独立源解释器证明。E9／FM3 仍保留已有 Python 静态／合成检查，本 Go 子包没有把它们补成未经实际验收的生产入口。PSG volume 的实际请求与连续 driver 的 gating 由父分支完整 oracle 另验，不能把本表解释器的 22,528 个 tick 冒充全部 175,674 次原读输出已经在 Go 连续通过。

## 主 driver 需要保留的边界

- Patch 输出不包含最终 B4 pan／AMS／FMS、硬件通道／port 路由和 FM6 与 DAC 的竞争；这些归父 driver／声部仲裁 owner。
- Ordinary 音量使用传入 patch 的 carrier。原程序 TL 来源是 selected instrument，而 mask 来源是 cached algorithm；父 driver 不能在尚未加载或被 claim 抑制期间把两种语义错误合并。
- 禁用声像不推进状态。原 E4 mode 0 只修改 mode；替换成 disabled 时不能无条件调用 `InitialPanState` 清掉旧计数。
- 原 F5／F1 不直接清 envelope 游标，note cleanup 和 tie 决定 reset。本轮另核查了 4,080 次 F5 到下一 note outcome，没有遇到 selection-then-tied-note。未到达的跨定义保留 phase 组合仍是明确边界，不通过另存 raw cursor 解决。
- 参数包不抑制被 claim、note-off 或 gate 禁用的硬件写入；caller 按原顺序应用这些条件。参数计算与最终可听硬件状态是不同阶段。

F5 审计由 [score_param_selection_audit.py](../../../scripts/yuyuhakusho/score_param_selection_audit.py) 复跑，结果为 [selection-audit.json](../../../games/yuyuhakusho/verification/audio-score-param/selection-audit.json)。原源位置只出现在研究审计，不进入生产状态。

## 复跑与保留

在 worktree 根目录运行，Go 的缓存、临时文件和 telemetry 全部留在 worktree：

```sh
TEST_TELEMETRY_DIR="$PWD/.local/go-telemetry" \
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -json -count=1 ./games/yuyuhakusho/audio/score/param
```

Node／Wasm 的实际命令与环境见 `verification/audio-score-param/wasm-command.json`。同一冻结 fixture 嵌入测试，所以 Go／Wasm 检查不依赖本机 ROM、原 core 或私有 trace。重新生成 fixture 才需要这些原始证据：

```sh
python3 -B scripts/yuyuhakusho/score_param_fixture.py
python3 -B scripts/yuyuhakusho/score_param_selection_audit.py
```

本分工没有产生需删除的临时副本、浏览器 profile 或工作树。保留单份 312,236-byte 压缩 fixture、生成来源、真实 red、最终原生／Wasm／vet 日志及审计；共享且仍在使用的 Go cache／tmp 不做删除。没有提交、发布或修改公共平台、Board、README、PROGRESS。
