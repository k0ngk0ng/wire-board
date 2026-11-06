# 世界图形 DMA 与跨帧中断

已实现关闭显示时的 H40 DMA 分段计算，并按原版真实传输验证整项耗时、扫描线推进和跨帧行为。计算使用每项长度、目标、当前 VDP 阶段和后台等待标志，未使用原 CPU、原 RAM 或记录的结束时刻作为生产输入。

原版世界包装会跨视频帧。DMA 结束后恢复 SR 可能立即进入垂直中断，再返回传输循环。当前计算器尚未接入完整世界包装的跨帧事务，垂直中断处理本身也仍待计价；完整比赛音频继续关闭。

## 已验证的行为

`videoclock.Clock.AdvanceForcedBlankDMA` 返回一次实际 VDP 批次的剩余字数、结束时刻及后台总线等待效应。VRAM 与 CRAM／VSRAM 使用各自的实际速率；垂直消隐期与普通扫描线使用各自的预算。函数不自行推进 CPU、扫描线、传输队列或音频帧。

主机以及已经等待总线的后台会被 VDP 后续批次重设时刻。跨批次后不能重新加上发起指令的尾部周期，也不能用单个累加等待值代替这些操作。恰好用完预算时保留原严格比较；refresh 越过行尾时保留参考实现的 uint32 乘法行为。

本次原生对照包括两组：

- 两个固定核心各执行 2,592 个真实 DMA 更新。覆盖 VRAM／CRAM／VSRAM、普通行／垂直消隐、多个行位置和行尾 refresh 越界、1 至 65,536 字、预算相邻值，以及后台已经等待／未等待。两个核心的全部结果一致，各自运行前的原有 2,400 帧视频／RAM 对照一致。
- 19 次真实世界包装调用保留 2,268 个 CPU 步骤、89 次图形传输和完整 I/O。11 次自然调用与 8 次显式私有入口调用中，6 次整个包装跨帧，4 次 DMA 项本身跨帧；其余两次在传输项之间跨帧。4 次原版忙分支直接返回。采集时原有 2,400 帧视频／RAM／PCM 对照一致；私有入口分别在观察器关闭与开启时执行，完整结果一致。

真实传输测试从每项实际入口和 typed 长度／目标出发，通过生产计算器及现有 VDP 时钟计算完整时长，再比较原版结束时刻、CPU 恢复时刻和完整 VDP 状态。中途及最终的原版 DMA 结束时刻只用于断言，没有回填为输入。

另有 6 次真实恢复 SR 后接受垂直 IRQ 的对照：确认先按实际 CPU 相位接受 IRQ，再计入恢复 SR 操作的固定尾部。此项验证既有异常入口函数的使用位置，不代表已经实现垂直中断处理体或整个世界包装。

| 当前时钟范围 | 顶层测试 | 含子项记录 | 结果 |
| --- | ---: | ---: | --- |
| 原生竞态 | 11 | 39 | 通过，2.99 秒 |
| 实际 Node／Wasm | 11 | 39 | 通过，1.16 秒 |

两个后端通过名称集合一致，没有跳过测试。既有 VDP 测试一并回归，相关 vet、格式和差异检查通过。未修改的整局规则、其他游戏及网络性能矩阵未重复运行。

## 冻结、失败与修正

[失败范围](FAILURES.md)、测试、原生输入及采集源在计算器实现前冻结，见 [test-first.json](test-first.json)。缺 API、首次真实失败与后续结果全部保留。

首次生产实现错误地把行尾越界预算按普通有符号乘除处理。原版的 unsigned rate 使乘积按 uint32 运算，分段矩阵据此失败。仅修正生产计算，原版输出保持原样。

准备过程有两类真实修正，并保留了原文件及准确差异：

- 两次私有传输实际在新帧第一行、垂直 IRQ 信号之前开始。测试适配器原来将 Line 0 标成非法 Scanlines，现按已有时钟契约还原为 PreVertical。所有原版时刻和状态断言不变，见[第一行适配记录](first-blank-adapter.json)。
- `vdp_reset` 会把参考核心的画面高度重置为 192，初版分段取证没有恢复声明的 224 行。取证现显式还原 320×224 后调用原始函数。2,592 行输入不变，全部非垂直消隐输出保持原字节，588 行垂直消隐结果更新；旧 192 行结果、源文件和失败记录仍保留。测试同时分开统计 6 次包装跨帧与 4 次 DMA 项跨帧，继续要求全部 89 次真实传输。见[画面高度与计数修正记录](viewport-and-count-correction.json)。真实世界包装的原生 fixture 未改动。

最初取证还假定包装在一个视频帧内完成，因此在第一个真实跨帧案例停止。原尝试及日志保留，采集改为继续同一次调用，并记录每个事件所属的实际视频帧。另一次尝试无法访问核心内的 static 变量，随后使用公开 VDP reset／control 接口初始化，未修改或重编译原核心。

## 复跑

在仓库根目录执行，输出目录必须位于仓库内且尚不存在。验证器会保存命令、环境及输出摘要，并把 Go 临时文件、缓存、模块和遥测留在工作区。

```sh
python3 -B scripts/yuyuhakusho/verify_world_dma.py --mode native-race --out .local/md-yuyuhakusho/dma-recheck-race
python3 -B scripts/yuyuhakusho/verify_world_dma.py --mode wasm --out .local/md-yuyuhakusho/dma-recheck-wasm
python3 -B scripts/yuyuhakusho/verify_world_dma.py --mode vet --out .local/md-yuyuhakusho/dma-recheck-vet
python3 -B scripts/yuyuhakusho/verify_world_dma.py --audit
```

各命令应返回退出码 0。`--audit` 校验当前源码、冻结历史、原版数据和日志绑定。原生重新采集使用现有固定 ROM／核心及尚不存在的仓库内目录：

```sh
python3 -B scripts/yuyuhakusho/capture_world_dma_segments.py --out .local/md-yuyuhakusho/dma-segment-recapture
python3 -B scripts/yuyuhakusho/capture_battle_world_dma.py --out .local/md-yuyuhakusho/dma-world-recapture
```

## 下一步

把逐项 DMA 推进接入同一主时钟与后台播放器，完成跨帧音频结算、后台总线等待、实际垂直中断处理及世界包装的恢复。原生记录已经包含这些交错路径；不要再用单帧入口完成或每项耗时正确代替整次调用完成。此后仍须完成其他世界工作、联合存档、平台 PCM 流及完整浏览器比赛验收。
