# 原声音控制状态

`audiostate` 是独立的 Go 声音控制状态机，用于保留原音序／效果计算的可观察状态，尤其是被特殊球比较误读的 `$00FF` 工作字节。包内没有播放器、CPU／APU 模拟器、PCM 生成或按曲目帧数查表的预录结果；浏览器音乐仍由平台音频接口负责。

## 接口与状态

```go
state := audiostate.New(carriedWork)
if err := state.Request(0); err != nil { /* 停止当前推进并报告 */ }
if err := state.Request(61); err != nil { /* 停止当前推进并报告 */ }
if err := state.Tick(); err != nil { /* 停止当前推进并报告 */ }
work := state.WorkByte
```

`Request` 同步应用原声音 ID／控制命令；`Tick` 对应一次完整原声音 IRQ 更新。调用者必须提供原事件顺序，不能让浏览器媒体进度、玩家静音、音量或网络到达时间驱动本状态。包本身不假设声音 IRQ 发生在世界逻辑之前或之后，也不把 NMI 当作声音更新入口。

`New` 接收调用点已经具有的工作字节。原 reset 命令不清除这个字节，因此不能在每次 reset、重连或读档时将它归零。`State` 使用命名字段保存 9 个通道、节拍相位、包络、颤音、效果优先级、控制标志和必要的输出寄存器影子；标准 JSON 可完整保存和恢复。寄存器影子仅用于恢复原控制分支，不写设备。

支持源数据中的声音 1～76，以及 0、251～255 控制命令。未知请求、缺失音序、包络或打击乐数据、未支持的命令用法和指令流超限均返回错误；不返回默认工作字节。合法调用中的错误也不能被调用者忽略并继续对局。当前尚未定义把任意损坏的 JSON 状态修复为合法对局的机制。

重要兼容细节：

- 外部正声音请求先写工作字节 128，再判断效果优先级；被拒绝的效果也可能改变该字节。
- 253 暂停音乐部分，但效果通道仍推进；255 根据保存音乐恢复，并按原规则清理／重载通道。
- 效果通道按 8→4 推进，音乐参数按 2→0 更新，节拍进位后才推进音乐音序 3→0。
- 原整数比例乘法中乘数 0 表示 256／256；被乘数 0 时不执行工作字节减 1。
- 命令分派写入的原处理地址高字节来自保留的来源元数据，不使用 Go 函数地址。
- 结束效果并恢复被覆盖的音乐时，原程序留下的音序偏移变化也被保留。

## 数据来源

`source-data.json` 是从已固定 SHA 的原 ROM 离线解析的音乐命令图与音色参数，包含 76 个声音入口、73 个唯一头、9,209 个按通道解析的命令、6 个音色、2 个包络和 6 个打击乐表引用。它不包含 ROM 可执行代码、运行时 RAM 快照或预录工作字节时间表。

`source-data-source.json` 保留原 ROM、数据和生成脚本摘要。源字节地址仅作为原序列标识与兼容元数据；产品不以通用内存读写接口执行这些地址。生成器为 `scripts/basketball/export_audio_state_data.py`。

## 先行 oracle 与验证

实现前先列出失败场景并导出原指令 oracle，再编写 `state_test.go`；缺少实现的失败记录在 `.local/basketball/audio/state/before.log`。开发 oracle 使用已有 py65 执行未改动的原声音驱动，不读取 APU 状态；该工具只在研究脚本中，产品包不依赖它。

`testdata/original-driver.json.gz` 有 87 个场景、77,369 次请求／更新。每一步同时核对工作字节和 `$0700..07E6` 的完整原控制状态。RAM 投影只存在于测试，运行时使用命名字段。覆盖包括：

- 八场音乐各先推进 90 次，再请求效果 30 并推进 180 次；后段 1,440 个工作字节与既有特殊球反例 trace 全部相同。
- 原声音 1～76 各 600 次更新；有限结局 75 的 9,000 次更新与 76 前奏／循环的 20,000 次更新。
- reset 保留工作字节、暂停后效果继续、同帧顺序请求、优先级、同曲和重新加载的恢复、周期重触发关闭。
- JSON 保存后继续 300 次更新不分歧；未知请求和坏音序位置显式报错。

独立来源绑定另外核对 sound 30、music 61 各 597 个已有 FCEUX 状态，与 oracle 逐字节相同。来源记录在 `.local/basketball/audio/state/source-binding.json`；这 1,194 个状态不需要重新采样或录音。

```sh
source scripts/env.sh
go test ./games/basketball/audiostate
go test -race ./games/basketball/audiostate
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_audio_state_evidence.py
```

该包的普通及竞态测试已通过，记录为 `.local/basketball/audio/state/implementation-second.log`、`race.log`。原始完整 oracle 可由 `export_audio_state_oracle.py` 重新生成；正常验证直接复用已保存夹具。

## 接线边界

当前没有修改 world／runtime、播放器或播放清单。原 IRQ 相对世界推进的位置、同帧声音请求的位置及 Court 2 `$BDD2` 的自然链路仍由接线工作单独验证。因此，本包通过原控制状态对照并不代表特殊球问题已经闭合。

控制状态调查、原写入点和自然时序证据见 `docs/research/nes-basketball-audio-work-byte.md`。现有本地媒体压缩、许可和人工试听状态不因本包通过测试而改变。
