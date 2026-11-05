# Z80 bank 访问与声音 phase 的直接时基证据

两条完整原版输入流的 851,037 次 Z80 ROM bank 读取已逐次记录，并与声音写入、IRQ 和场地语义块共享同一原执行序号。baseline 原有 21,963 个普通场地块中的 6,710 个残差，现均由同一区间实际发生的 70／77 master cycle 总线占用解释；pause 流的 14,179 个普通块中，4,422 个残差也全部归零。

这项结果补齐了[上一阶段](AUDIO_PHASE_BUDGET.md)「残差与总线占用强相关，但尚未逐次观察」的缺口。它仍以原版事件时刻为输入，不表示独立世界／IRQ 调度已完成。原帧尾消费反例及旧纯 CPU 模型保持原样，未扩大成本常数、删除反例或改变 producer。

## 观察与原控制

观察器 [phase_clock_bridge.c](../../../scripts/yuyuhakusho/phase_clock_bridge.c) 复用已冻结 `audio_phase_budget_bridge.c` 的声音／IRQ／语义 recorder，只在当前进程中包裹原 `z80_readmem`／`z80_writemem` 函数指针。wrapper 调用原 handler，并在同一 ledger 记录访问前后的 68000／Z80 周期、DMA 参数、等待标记及映射地址。不修改固定 core、指令、ROM、数据或原游戏调度。

| 检查 | baseline | 合法暂停流 |
| --- | ---: | ---: |
| 完整视频步骤 | 5,500 | 3,602 |
| bank 读取 | 512,657 | 338,380 |
| 延长 68000 70 周期 | 366,235 | 241,683 |
| 延长 68000 77 周期 | 146,422 | 96,697 |
| 先等到 DMA 区间末端的读取 | 211 | 146 |
| 场地普通语义块 | 21,963 | 14,179 |
| 原纯 CPU 残差非零的块 | 6,710 | 4,422 |
| 减去同区间直接 bank 占用后仍有残差 | 0 | 0 |

全部 9,102 步 RAM、寄存器、视频、PCM 摘要与样本数逐步等于既有未观察控制。去掉新 bank marker 后，原声音写入、IRQ、callback 和语义事件逐项等于冻结 compact ledger；没有因本次采集改动旧 recorder。所有读取值也逐次等于固定 ROM 对应字节。两条流另以新进程完整重采，压缩事件与逐帧 PCM／state 证据逐字节相同。

观察到的 bank 读取地址范围分别为 `048000..0b6d0e` 与 `048000..0a1e06`，均在原 ROM。自然流未观察到 bank 写或 Z80 的 `7Fxx` VDP 访问；wrapper 包含这些入口并不代表它们已自然验收。

## 固定参考 profile 的规则

原 handler 在检查 `cycle < dma_end && mode < 2` 后，先将 Z80 相位推进到 DMA 末端。若此时 `dma_length` 非零，再置等待标记。随后以等待后的当前帧相位计算：

```text
main_delay = floor(((cycle_after_DMA mod 7) + 72) / 7) × 7
z80_resume = cycle_after_DMA + 45
```

这 851,037 笔原访问全部满足该算术。固定 core 的注释把 68000 wait-state 明确写作经验近似，并说明该值针对特定测试 ROM／显示问题作过调整。因此这里恢复的是已固定参考配置的行为，不是实测真实 MD 硬件的宣称。

不能直接用全局 master 时间的余数替代当前帧相位，因为 `896040 mod 7 = 5`。同一 corpus 将访问时间错误增加一个帧原点时，有 486,206 次算出不同的延长；Go／Wasm 的冻结检查保留了这个反例。

共有 281 次 bank 读取开始于参考视频帧尾或之后，原 raw cycle 全部保留。不能把它们裁到帧尾。它们与 [DAC DSP 的 frame flush／lookahead](AUDIO_DAC_DSP.md) 是不同层次：这里是 memory handler 的周期效果，DSP 自己仍需保持已推进的芯片状态与丢弃输出的区别。

## 纯 Go 接口与状态归属

新增 [audio/phaseclock](../../../games/yuyuhakusho/audio/phaseclock/README.md) 仅提供 `ResolveBankAccess(frameCycle, DMA) (Effect, error)`。输入是已发生一次 memory operation 的当前帧相位与 DMA 区间，输出是访问相位、Z80 恢复周期、68000 延长和请求置位的 wait flag。它不持久保存 Z80／68000 时钟、ROM 游标、队列或原寄存器。

PCM player 保持其预取、bank 读取与 DAC 事件的唯一 owner。每一次实际读取，包括 dummy／prefetch，都应在语义发生处应用本次 effect；不能在完整解码字节结束后统一加上所有 DMA 等待，因为这会错过半轮后续读取与 DAC 的时刻。未来世界／driver 调度引用同一 bus 规则，不能各自持有第二份原 RAM 或时钟影子。

此接口尚未由世界、平台或播放器的生产调用者接线。当前不是声音起播实现，也不选择 callback 前／后哪一个 barrier。

## 测试先行与边界

[失败场景](AUDIO_PHASE_CLOCK_FAILURES.md)与检查器先于新观察器冻结。对旧 compact trace 的真实 red 明确指出「没有逐次 bank 事件」；baseline 同时保留原 6,710 个残差。其后同一检查器对新采集取得 0 残差。报告仅将全量 bus 检查结果保存为有序摘要，全部输入／原结果仍在原始采集与 portable corpus，未裁除失败。

Go helper 的声明、851,037 条原输入／输出夹具及三项测试也先于 `bus.go`：首次真实编译 red 为缺少 `DMA`／`Effect`／`ResolveBankAccess`。随后原生 race、Wasm 与 vet 通过，完整记录见[验收索引](../../../games/yuyuhakusho/verification/phase-clock/validation.json)。夹具为每条原记录的输入和实测输出，未使用模型生成 expected。

自然两流只到达 DMA mode 0／1、原 zstate 1，DMA 等待期间的 `dma_length` 均为 0。先行单元测试中的 mode 2／3、非零 `Remaining` 置位、保留已置位 flag 以及 uint32 合法末端，另通过两个原生 core 的各 11 个声明私有 handler 初态交叉核对。它们只读取原 ROM，不推进 CPU、视频或 PCM；不能计入自然对局步骤。

## 尚未解决的调度

已直接解释普通场地块的总线延长，不等于已经知道未来读取何时发生。完整独立调度仍需自主 PCM／prefetch 事件、68k 对 Z80 的 bus hold、driver／score 语义成本、场地 camera／除法／其他 callback、VDP DMA／FIFO 与 IRQ 接受时刻。

含 IRQ 或 callback 自身硬件访问的场地块继续单列，不用纯 row-loop 公式强行解释。新 ledger 将 bank 与 IRQ 放在同一真实顺序中，但没有从纯规则预言未来 IRQ。旧 callback-before／after 的归约仍是给定观察流的存在性结论；不能把原选择表送入运行时。

## 复跑

无需原 ROM 即可重复 portable native／Wasm helper 检查，命令见 `verification/phase-clock/go-commands.json`。重新采原执行需要已固定 ROM、两份参考 core、clang 及仓库内环境目录：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_phase_clock.py \
  --out .local/md-yuyuhakusho/phase-clock-review
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/check_phase_clock.py \
  --source .local/md-yuyuhakusho/phase-clock-review/phase-clock-baseline.json.gz \
  --out .local/md-yuyuhakusho/phase-clock-review/baseline-check.json.gz
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_phase_clock_boundaries.py \
  --out .local/md-yuyuhakusho/phase-clock-boundary-review
```

原采集保留在 `.local/md-yuyuhakusho/phase-clock-native/`，portable 夹具及直接归因结果保留在游戏目录；原始路径、SHA、完整先行 red、重采等值与清理范围由验收索引绑定。本分工没有修改其他音频模块、世界引擎、平台页面、研究总索引或进度百分比，也未提交、发布或上传。
