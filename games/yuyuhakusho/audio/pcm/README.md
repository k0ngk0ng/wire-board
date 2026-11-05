# DPCM4 与双槽 DAC 算术

本包只实现直接 typed 算术：`NewDecoder()`、`DecodeByte(state, encoded)` 和 `MixDual(a, b)`。不执行 Z80、CPU／RAM VM、固件或 ROM，也不决定播放时间、载入、结束或过滤。

`DecoderState.Accumulator` 是唯一状态。初始化为 `128`，先解码高 nibble，再解码低 nibble；两步均按 8 bit 回绕。例：初始状态解码 `0x32` 得到 `[132,134]`，下一状态为 `134`。非活动双槽贡献 `128`；相加后先在 DAC 的 8-bit 层限幅，例如 `MixDual(200,200)=255`。

测试期望来自原固件的完整母表及真实原程序的输出，不由实现函数自身生成。4 个原版场景覆盖 28,510 个编码字节、57,020 个解码值和 41,345 次滤波前 DAC 写，全部相同；其中 476 次触发双槽 DAC 限幅。原生竞态和 Node／Wasm 的四项测试通过。

```sh
TEST_TELEMETRY_DIR="$PWD/.local/go-telemetry" \
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -json -count=1 ./games/yuyuhakusho/audio/pcm
```

原事件顺序仅为测试 oracle。按原 decode／mix／DAC 阶段推进的数值相同，不证明独立 cadence、bank 预取、总线握手、YM2612、filter／resampler 或平台播放时刻已经恢复。详细证据见 [PCM 模型研究](../../../../docs/research/md-yuyuhakusho/AUDIO_PCM_MODEL.md)和[验收索引](../../verification/audio-pcm-model/validation.json)。
