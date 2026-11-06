# 战斗采样中断前置流程与真实声音调用

实际 RAM 采样中断前置流程已接到完整声音调度／驱动调用。垂直计数等待、寄存器保存、控制器扫描和后台推进共享同一个主时钟、refresh 与 CPU 操作预算；后续世界失败或预算不足时，真实音频和借用的 queue／pause 一起撤销。

`PrepareBattleSampleIRQ` 运行前置流程，结束于实际声音调度入口。`AdvanceBattleSampleIRQ` 继续调用原有完整声音实现；`session.BeginBattleSampleIRQ` 在实际音频分支上组合两者。返回的 `AwaitingReturn` 明确表示外层寄存器恢复／RTE 尚未执行，调用方此时不能恢复 World。音频存档未增加 World 队列、主时钟、原始寄存器、原 RAM 或操作历史。

## 实现与时间顺序

- `MainWorkOperation` 由原场景工作原语提取，保留旧 `ArenaWorkOperation` API。临时寄存器保存为 112 个内部 CPU 周期、内部 refresh 跳过、8 个固定周期，不能合并为普通 120 周期；旧 792 个场景／摄像机调用继续逐步验证。
- `ScanBattleControllers` 完整采样两个虚拟端口，支持直连 MD6 与四 MD6 的 TeamPlayer，覆盖双直连、转接器加直连、直连加转接器和双转接器。设备信号按 held 位集、连接方式和扫描阶段生成；仅在单次调用中保留临时引脚状态，不加载程序或时间表。
- 控制器结果作为待消费的采样阶段输出，未在 IRQ 中提前覆盖 World 的归一化控制器。MD3／MD6 识别与 Mode／X／Y／Z 位保留按原顺序处理。
- 垂直计数使用实际批次状态，在当前工作结束后推进扫描线；同一指令的过冲按原 HVC 读取规则处理。先完成主机批次，再对相同批次目标推进辅助播放器，把真实 `M68KDelay` 加回唯一主时钟，然后推进 VDP。
- `service.before` 是临时批次协调回调，前置流程和后续整个声音调用都使用它。只在 YM 总线请求时才同步后台，或在声音调用结束后统一补后台进度，都会遗漏行间的银行争用。

当前窗口限定在同一个未 flush 的参考帧，且没有 VDP DMA。辅助播放器在入口时必须已推进至当前扫描线起点；不能把从零开始的全部历史争用加到本次 IRQ。跨帧 flush、VDP DMA 和更完整的主循环接入仍需继续完成。

## 原版证据与测试

[失败范围](FAILURES.md)、新测试、输入与原生预期先于相应实现冻结；见 [控制器与时钟先行记录](controller-test-first.json)和[前置层先行记录](prefix-test-first.json)。真实缺 API 编译失败保留。全部新测试与已冻结原生预期保持原样，首轮实现通过。

1. 两个固定参考核心分别验证 2,400 个冷启动视频步，再对控制器执行观察器关闭／开启两遍。288 个稳定案例与 288 个冷复位案例，完整步骤、80-byte 结果、RAM 摘要和 IO 完全一致；稳定／冷复位各 112,608／108,576 个步骤。
2. 自然原版 2,068..2,074、2,393..2,400 共 15 次真实采样中断前置流程，在 2,400 个视频／RAM／PCM 对照全部相同的前提下被动记录；另有 8 个明确私有输入的连接方式 × display-ready 案例，观察器开关的完整帧结果相同。
3. 23 个完整前置案例包含 14,282 个 CPU 步骤、3,816 个 IO 操作和 285 个外部批次反馈。Go 逐项匹配时钟、refresh、VDP、pin、mask、阶段和读取／写入值。
4. 使用真实 PCM／混合器／声音驱动的联合测试覆盖共享预算、JSON 恢复，以及后续 World 失败时的整体回滚。旧 122 个完整声音 IRQ、普通 IRQ 接受、场景内部工作及既有真实声音事务继续回归。

| 运行范围 | 顶层测试 | 含子项记录 | 结果 |
| --- | ---: | ---: | --- |
| 原生竞态相关回归 | 16 | 970 | 通过，62.24 秒 |
| 实际 Node／Wasm 相关回归 | 16 | 970 | 通过，44.92 秒 |

两端通过的测试名称集合一致。相关 vet、格式、链接和差异检查通过，正式输入、源码、命令、环境及压缩日志由 validation.json 绑定。原有测试、数据、夹具和原生预期没有改写；更早的源码绑定清单作为历史证据保留。

原版前置层对照中的 285 个后台反馈是**明确给定的原生外部输入**：按真实批次目标记录其主时钟延迟。这个有限证明不等于已从新建比赛独立生成原版全部 PCM／DMA 反馈。实际运行入口使用同一个真实混合器的 `RunUntil`，联合测试另验证其事务组合；整局独立输入／世界／后台时间仍未完成。

## 冷复位与稳定扫描的准备修正

首次控制器采集使用 `io_reset` 的零 data／control，原版会多消耗一次 TH 上升沿，把直连六键暂时识别为三键。该结果本身正确，但最初采集范围文字误称为稳定扫描。原源文件、原 fixture 和说明均保留为 cold-io-reset 证据；没有修改其结果。

新采集另外声明先前完整扫描的结束状态：直连端口 data／control 为 40，TeamPlayer 为 60，协议计数经扫描线间隔复位。稳定六键与冷复位三键分支分别纳入原版和 Go 对照。准备修正发生在生产实现前，见 [说明与准确 diff](controller-settled-setup.json)。

目前物理采样输入域为合法十字键方向；原有平台 `ValidateControllerButtons`／`SampleController` 的 12-bit 逻辑输入契约未改。对向按键等平台输入如何进入完整物理采样模型，仍需在主循环接入前补齐，不能直接把未支持输入导致的错误带入正式比赛。

## 复跑与剩余工作

输出目录使用工作区内尚不存在的目录：

```sh
python3 -B scripts/yuyuhakusho/verify_battle_sample_irq.py --mode native-race --out .local/md-yuyuhakusho/sample-recheck-race
python3 -B scripts/yuyuhakusho/verify_battle_sample_irq.py --mode wasm --out .local/md-yuyuhakusho/sample-recheck-wasm
python3 -B scripts/yuyuhakusho/verify_battle_sample_irq.py --mode vet --out .local/md-yuyuhakusho/sample-recheck-vet
python3 -B scripts/yuyuhakusho/verify_battle_sample_irq.py --audit
```

重新采集与导出只使用已固定的研究核心／ROM：

```sh
python3 -B scripts/yuyuhakusho/capture_controller_scan_work.py --out .local/md-yuyuhakusho/controller-recapture
python3 -B scripts/yuyuhakusho/capture_battle_sample_prefix.py --out .local/md-yuyuhakusho/prefix-recapture
python3 -B scripts/yuyuhakusho/extract_battle_prefix_fixture.py --out .local/md-yuyuhakusho/prefix-reexport
```

仍须实现寄存器恢复／RTE、立即跟随的下一阶段 IRQ、屏蔽变化与延迟接受，再补齐其他世界工作、跨帧／DMA 和唯一主时钟下的联合帧存档。正式 PCM 输出、资源发布和完整浏览器对局听音验收继续待办。试玩服务仍为原构建，完整比赛音频未开启。此前无法复现的 Go/Wasm heap 异常也未据此宣称修复。
