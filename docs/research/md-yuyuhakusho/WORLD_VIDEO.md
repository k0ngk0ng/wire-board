# 固定 VDP 计数与 IRQ 信号层

`audio/videoclock` 已完成固定参考 NTSC、224 可见行、H40 非交错 profile 的最小状态层。原 CPU 决定执行批次、指令成本和中断接受，本层只按外部调用顺序推进视频计数、pending 和 IRQ 请求；尚未接入 `World`。

## 已验证的范围

原生只读观察从冷启动逐步对照冻结控制组，2,400 视频步骤的 RAM、寄存器、画面与 PCM 全部一致。保留比赛步骤 2,068～2,074、2,393～2,400 两个连续窗口，共 15 帧、7,757 个直接 scalar 状态、2,193 个 M68K 状态／HVC 读取。每个窗口只注入一次初态，之后按外部操作连续推进；Go 每次操作后均经过 JSON 保存恢复。

| 事实或关系 | 证据与限制 |
| --- | --- |
| 每行 3,420、每帧 896,040 master cycles，帧首为 VCounter 224 | 固定 core `system.c` 与原生 line/counter 状态；不能推广到 PAL、240 行或复位首帧 |
| 首空白行分成 VINT 前后两段，边沿为 788 | 固定 core 调用顺序；IO 可跨越批次目标，不按 raw CPU cycle 自动触发边沿 |
| 最后空白行重装 H counter；可见行先重装旧 Reg10 再进入 ISR | 源码关系与原生状态一致；给定外部写入后，窗口内 HINT 为 160,740／194,940／827,640 |
| M68K 状态读不清 HINT／VINT pending | 直接原生读取与后续状态；只验证返回值的 `0x88` 投影 |
| HVC 高字节使用 raw cycle；过冲只向下一行一次 | 2,178 个原生 HVC 读取；不实现 HCounter 像素计数或 HVC latch |
| ack 优先清 enabled VINT，否则清 HINT | 固定 core 源码与 60 个 handler entry 之后的原生状态；ack 调用本身未直接捕获 |
| HINT 按位 OR 请求，VINT／ack／使能更新按各自条件赋值 | core `m68k_update_irq`／`m68k_set_irq` 源码与独立先行边界测试；没有直接原生 CPU pin 一致性声明 |

原 ROM 的 68000 静态清单来自 direct Ghidra，不称 REA 原生反编译。相关来源为 `verification/world-audio-barriers/{static,mode-setup}/vdp-and-irq-setup.txt`、`static/world-frame-irq.txt`，原生身份与 SHA 见 [capture.json](../../../games/yuyuhakusho/verification/world-video/native/capture.json)。

## 调用与状态归属

`BeginFrame` → 外部执行首批次 → `VerticalEdge` → 外部执行本行余段 → 重复 `NextLine`，直到 `BetweenFrames`。寄存器写和 `Acknowledge` 必须保留实际外部顺序。`State` 只保留阶段、行序、H counter／reload、使能、display 与 pending，不保存 CPU pin、PC、原 RAM、完整寄存器阵列或原时间表。

`Effect.SetIRQ` 表示替换外部 CPU pin，`Effect.UpdateIRQ` 表示将 `Level` 按位 OR 到 pin，两者互斥。`Deferred` 由外部 CPU owner 处理原延后服务语义；本层不代替执行下一条指令，也不由最高 pending 推导 pin。

首次实现把 HINT 表达成赋值；源码复核后，先取得「原 pin 6 被错误降为 4」的真实语义 red，再冻结测试并增加 `UpdateIRQ`。旧实现、旧测试、语义 red 与缺 API red 均保留在 [merge-test-first.json](../../../games/yuyuhakusho/verification/world-video/merge-test-first.json) 的来源链中，未改写初次结果。

## 验收与未完成项

Native race、实际 Node/Wasm 各通过 7 个主测试和 2 个原生窗口子测试，`go vet` 通过；来源、命令、环境与日志 SHA 保存在 [go-v2](../../../games/yuyuhakusho/verification/world-video/go-v2/native-command.json)。离线复跑不需要 ROM 或模拟器：

```sh
python3 -B scripts/yuyuhakusho/world-video-check-native.py
python3 -B scripts/yuyuhakusho/world-video-verify.py --mode native --out "$PWD/games/yuyuhakusho/verification/world-video/recheck"
python3 -B scripts/yuyuhakusho/world-video-verify.py --mode wasm --out "$PWD/games/yuyuhakusho/verification/world-video/recheck"
python3 -B scripts/yuyuhakusho/world-video-verify.py --mode vet --out "$PWD/games/yuyuhakusho/verification/world-video/recheck"
```

第二次原采试图委派 CPU ack callback，在第 2,068 步 attach 校验时安全拒绝；没有生成有效新捕获。固定 core 使用 `OPT_SPECIFY_HANDLER` 直接调用 `vdp_68k_irq_ack`，不启用 callback pointer。失败日志及执行源保留，未修改 core、重建研究 core 或继续扩展 instrumentation。

独立 CPU 接受、中断内部前后 pin、复位 bootstrap、world 操作成本、world→IRQ 排程、完整 VDP 状态、任意模式与最终音频均未证明。通过给定外部 IO／ack 的窗口不能替代这些组合验收。
