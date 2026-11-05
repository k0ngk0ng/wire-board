# 独立 DPCM 算术与滤波前 DAC 证据

已恢复独立 Go `audio/pcm` 的 DPCM4 解码和双槽 8-bit DAC 限幅算术。真实原程序的 28,510 个编码字节从各自初始 `128` 状态连续推进，57,020 个解码值及 41,345 次滤波前 DAC 值全部一致。原生竞态与 Node／Wasm 四项检查通过。

当前尚未恢复独立事件时钟、预取、总线握手或最终 PCM 过滤。测试中的原阶段与时间只作为证据，不能带入生产回放。机器索引见 [audio-pcm-model/validation.json](../../../games/yuyuhakusho/verification/audio-pcm-model/validation.json)，先行失败范围见 [AUDIO_PCM_MODEL_FAILURES.md](AUDIO_PCM_MODEL_FAILURES.md)。

## 编码、相加和限幅

固定 Z80 固件的 `A00/B00` 两组 256-entry 母表，分别按编码字节的低／高 nibble 展开同一 16 项 delta：

```text
0, 1, 2, 4, 8, 16, 32, 64, -128, -1, -2, -4, -8, -16, -32, -64
```

先处理高 nibble，再处理低 nibble；累加器和结果均为 8 bit，负值和溢出按原加法回绕。单模式原 `0116` 将累加器初始化为 `80`；双槽各自的累加器也从 `80` 开始。

`800..9FF` 的 512-entry 母表逐项恰好等于 `clamp(index-128,0,255)`。原双槽路径先把两个解码值相加，使用该表给出 DAC byte；无活动数据的槽贡献 `128`。这不是平均，也不是滤波后 16-bit WAV 相加。两个 delta 表和 mix 表合计 1,024 字节的 SHA-256 为 `be79e4eb4329b2f21dfda3d284a5b03afbd2dd8b1694c4366190072de0ef0fe8`。

生产包只保存 16 项数学 delta 常量和直接算术。`DecoderState` 只有一个 `Accumulator uint8`，没有固件、指令、寄存器或 RAM 镜像。命令、源数据位置、active 生命周期、两个槽及输出时间仍由未来驱动唯一拥有。

## 原版采样与观察器

新观察桥只在当前参考进程临时包装 `fm_write`、`z80_readmem` 和 `z80_writemem` 函数指针，参数／返回值原样委托给原 handler。它不修改原指令、固件、样本或 CPU 周期。每次恢复后安装、每条流结束后严格确认 wrapper 未被替换，再恢复原指针；事件数组溢出即失败。

采样复用已验证的静音检查点方法：固定合法输入预热至 2,400 步，明确私有清除世界生产者，经真实控制命令 `04` 等待 120 步静音。恢复总是补齐 `refresh_cycles`。这些是私有参考输入，不冒称正常玩法可达；未重复采集全部 134 个有限音效。

| 原场景 | 输入 | 视频步／端 | 编码字节 | DAC 写 | 8-bit mix 限幅 |
| --- | --- | ---: | ---: | ---: | ---: |
| `single-short` | 第 0 步 `06` | 80 | 2,570 | 5,217 | 0 |
| `pair-short` | 第 0 步 `06`，第 8 步 `7a` | 90 | 3,645 | 5,217 | 35 |
| `single-mode` | 第 0 步单 DAC 模式 `73` | 90 | 7,788 | 15,576 | 不走双槽表 |
| `pair-long` | 第 0 步 `1f`，第 8 步 `3f` | 160 | 14,507 | 15,335 | 441 |

原内核与带观察器内核共 840 个视频步骤的完整 RAM、视频、逐步样本数量和 PCM SHA 全部相同。共记录 205,219 个事件，包含 DAC／控制写、bank 读、编码与母表读、累加器与 busy 写；观察器溢出为 0。

母表来源是固定 ROM `393FE..39FFD` 的 3,072-byte 固件，SHA-256 `42ed1d2c4133d88d36c4024ff535a710f064dcb5d791b892eb3a07f6ded6ef27`。Z80／M68K ABI 仅在私有 observer 中使用，固定 Z80 context 为 88 字节；生产 Go 包不依赖这些内核结构。既有 [audio-z80.txt](static/audio-z80.txt) 来自 direct Ghidra，未宣称 REA 支持 raw 68000。

## 数值证明与独立时钟的区别

原程序实际编码读取处记录输入 byte 和旧累加器。原低 nibble 查表时的寄存器值给出真实高 nibble 结果；双槽累加器写入或单模式第二次 DAC 写给出真实低 nibble 结果。期望值不调用 Go 实现生成。

Go 的连续验收只在开头创建初始 `128` 状态，此后从不可变完整 payload 顺序读 byte，始终使用自己的上一状态；不会把原中间累加器注入 Go。两个槽的高／低结果再按原 decode／mix／DAC 的语义阶段顺序组成整个 DAC 值流，连额外的中性 DAC 输出也比较，41,345 次全部相等。

这里仍借用了原程序何时执行 decode、mix、DAC 的阶段触发，因此结论是完整 payload 与给定原顺序下的输出值正确，**不是独立播放时间预测已经完成**。测试保留的源 PC／cycle 只供审计，没有进入生产算法或回放表。

新原 pair 中有 476 次实际 DAC 8-bit 限幅。上阶段 [独立 WAV 叠放反例](AUDIO_DISPATCH_RESEARCH.md) 的 16-bit 相加虽然从未饱和，却不能重建原 PCM；两层限幅位置不同是已经定位的算法差异之一，不能据此把所有原波形误差都归因于限幅。启动相位、握手、预取及滤波仍有各自依赖。

## Bank 读与时间边界

四流共 31,799 次 Z80 bank 读，每次参考 core 均增加 Z80 `45` master cycles，并增加 M68K `70` 或 `77` master cycles。逐次严格符合固定 core 的经验公式：

```text
M68K += floor((Z80.cycles % 7 + 72) / 7) * 7
Z80  += 3 * 15
```

来源为固定 core 的 `core/memz80.c:z80_request_68k_bus_access`。该实现自己标为近似等待；不能当作真实硬件 pin-edge 或未经测量的跨机器物理时间。VDP DMA 还可能另行延迟 bank 访问，本批清空世界的流没有观察到超出 `45` 的等待。

Bank 读不能直接按编码 payload 长度计算：单模式 `73` 只解码 7,788 个 byte，却发生 11,071 次 bank 读，包含未直接形成输出的预取／dummy 读。这些读仍会影响 M68K 及可能的音频 IRQ 相位。

按原 NTSC 每步 896,040 master cycles 归一化后，DAC 事件严格有序，但相邻间隔并非一个常数。双模式常见 `6585/6600/6675/6765`，单模式 rate `3` 常见 `3300/3330/3345`，还有预取／握手相关长间隔。`constant-cadence-red.json` 保存将其当作固定周期的真实反例；独立 cadence／bank 日程仍未实现。

## 后续 YM／filter／resampler 层

本次 DAC byte 是进入原芯片之前的值，不是 44.1 kHz PCM。固定参考 core 还包含：8-bit DAC 到 14-bit 的 `((value-128)<<6)` 映射、离散 YM2612 的 ladder effect、声道 pan／其他 FM 声部、芯片采样相位、band-limited resampling、低通与输出量化。这些设置和状态必须单独固定并验收，不能把当前数值通过扩大成最终波形通过。

原 core 的 `sound.c` 在 YM 数据写前推进 FM，并用 blip delta 输出；`libretro.c` 的默认配置为 high-quality FM、离散 YM2612，低通范围 `0x9999`。`system.c` 按 `config.filter` 执行最终过滤。默认配置文字注释与真实分支可能不一致，后续应读取实际运行配置和同一初始音频状态，不能只凭注释设置滤波。

独立实现的下一层应接收 typed DAC／enable／pan 事件及明确的 master-clock 原点，并保存自己的规范 DSP／resampler 状态。给定原事件时刻的波形对照与从独立游戏规则推导事件时刻是两项证明；两者都不能靠播放原时间表、复位相位、裁切单音或携带 CPU／固件 VM 替代。

## 复跑

无需原模拟器的 Go 验证见 [audio/pcm/README.md](../../../games/yuyuhakusho/audio/pcm/README.md)。Wasm 的精确最小环境和命令见 `verification/audio-pcm-model/wasm-command.json`。原观察数据、母表和语义夹具在 worktree 内，可由以下入口复核：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_pcm_model_capture.py \
  --capture .local/md-yuyuhakusho/audio-pcm-model-first/capture.json \
  --out .local/md-yuyuhakusho/pcm-native-review.json
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/analyze_pcm_model_events.py \
  --capture .local/md-yuyuhakusho/audio-pcm-model-first \
  --out .local/md-yuyuhakusho/pcm-value-time-review.json
```

验证器接受 `capture.json` 文件；分析器接受目录。添加 `--expect-constant-cadence` 到分析器应返回非零。

重新编译被动桥：

```sh
mkdir -p .local/md-yuyuhakusho/audio-pcm-model-tools/tmp .local/md-yuyuhakusho/audio-pcm-model-tools/clang-cache
TMPDIR="$PWD/.local/md-yuyuhakusho/audio-pcm-model-tools/tmp" \
CLANG_MODULE_CACHE_PATH="$PWD/.local/md-yuyuhakusho/audio-pcm-model-tools/clang-cache" \
clang -dynamiclib -O2 -DLSB_FIRST=1 \
  -I .local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/core/z80 \
  scripts/yuyuhakusho/pcm_model_observer.c \
  -o .local/md-yuyuhakusho/audio-pcm-model-tools/pcm_model_observer.dylib
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_pcm_model.py \
  --out .local/md-yuyuhakusho/audio-pcm-model-review
```

所有输出仅位于当前 worktree。未修改原 metadata／WAV、仲裁、队列、scheduler、世界、平台或现有 producer；没有提交、上传或发布。

本阶段清理 6 个私有重复／临时目录，共 12,451,638 bytes；规范事件与 PCM、portable 夹具、原始失败、最终结果和观察桥保留。第二次完整原生采集及再次导出的语义夹具逐 byte 相同；带输出路径的 manifest 不宣称逐 byte 相同。工具目录检查另记录了未加工作区遥测变量的 `gofmt` 调用及其默认计数缓存可能性，未把未审计的外部缓存变化描述为已验证不存在。
