# PCM 命令 Go 状态机

`audio/pcmcommands` 将父分工已匹配原程序的音频语义落成显式 Go 状态机。它从 idle 开始，接收已接受的普通／音乐 intent 与 bus 窗口，自行产生启动、single 128-byte 预装、dual 描述转交、连续 DAC、预取、替换和自然结束。

固定 12 组向量的 expected 直接来自原 native 观察记录，不调用 Python 预测模型生成。每个参考 frame 均序列化自己的快照再恢复，随后逐项核对声音／总线事件、状态反馈、总线 grant／resume、PCM 与完整 DAC DSP。生产仅使用 typed 逻辑字段、固定 Go 音频阶段与静态原子延迟分区。

## 可核验边界

- immutable catalog 唯一拥有 encoded bytes，设备通过 `Length/ByteAt` adapter 读取，状态只引用逻辑 ID。
- 普通 intent 的目标与 `StartDual` 已由上层按 sampled 状态决定；音乐接受判断也留在 score。设备只执行提交与生命周期。
- `Frame` 与执行 frontier `Time` 分别保留 flush 与 CPU-independent 语义进度；bus hold 可以跨 frame，不复制原 uint32 下溢 counter。
- 源预取计数、已提交的 `Produced`、解码累加器及暂存混音有各自流水作用。没有隐藏 Go 协程栈或原输入历史。
- 元数据锁存期间的两类 opaque-resource 混合发布明确原子拒绝；不声称支持任意 ROM 地址组合。
- 完整 B6 pan byte 保留；DAC-only adapter 自行取高位。

## 测试先行与真实修正

[失败清单](AUDIO_PCM_COMMAND_GO_FAILURES.md)、original-only exporter、12 组 portable 向量和 API E2E 先于实现冻结。实际缺失 `Status/Config/New` 的 red 保存在 `verification/audio-pcm-command-go/api-symbol-red.jsonl`。

第一轮 Go 移植在 Control64 的描述转交路径漏掉 10 T-state 的分支跳转。原失败首先出现在 dual 的高／低输出之间；其坏版源与失败日志保留，修复仅补该静态跳转。原 12 组向量和断言未改动。

[边界复核失败清单](AUDIO_PCM_COMMAND_GO_REVIEW_FAILURES.md)与独立回归先于后续拒绝修复。初版接受缺少 single 上下文、inactive part 越界、空循环剩余量和无 dual mode 的 phase，以及两类交叉描述发布。它们各有实际 red 和修复前源，拒绝保持整个 `AdvanceFrame` 原子。

## 复跑

源码 API、状态与输入限制见 [包说明](../../../games/yuyuhakusho/audio/pcmcommands/README.md)。Go 测试无需 ROM：

```sh
TEST_TELEMETRY_DIR="$PWD/.local/go-telemetry" \
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -json -count=1 ./games/yuyuhakusho/audio/pcmcommands
```

本组不修改旧 `pcmplayer`、`audio/pcm`、`audio/dsp`、世界、公共平台或研究总索引。硬复位、全 YM 控制和更完整 DMA 原始向量由父分工独立补充；新增控制会保留原 12 组历史证明。
