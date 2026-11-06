# 采样中断返回与立即续接

采样中断现在可以在同一个主时钟、后台播放器和 CPU 预算上，完成声音调用后的寄存器恢复、RTE、紧随的 late-reload 中断，再返回原 World。`session.RunBattleSampleIRQ` 仅在这些阶段全部成功后提交真实音频及 queue／pause；预算不足或后续 World 失败时一起撤销。

原有 `BeginBattleSampleIRQ` 继续返回待完成状态，保持原接口语义。新的 `FinishBattleSampleIRQ` 是完整尾部操作：若返回后直接进入尚未计价的世界中断，它在第一条世界中断操作前报告 `PendingWorld`。完整会话入口将该状态转换为错误并丢弃分支，避免错误恢复角色更新。

## 顺序与状态

临时寄存器恢复的成本为 112 个内部 CPU 周期、内部 refresh 跳过、12 个固定周期。普通 Go 调用本身保留局部数据，运行时没有 CPU 寄存器文件、原始栈或返回 PC 存档。

RTE 在恢复 caller mask 后处理已挂起的 IRQ，再计入 RTE 自己的末尾 20 周期。确认 IRQ 使用现有 VDP owner，异常入口保留 CPU 相位及 master-cycle 余数。RTE 若跨过批次边界，还须先推进后台和扫描线，再进行下一批次的普通接受检查；两种接受位置分别处理。

立即进入 late-reload 分支时，按原顺序屏蔽 IRQ、检查阶段、写入 DF reload、推进到 World 阶段并再次 RTE。若此时已有下一次世界中断，不能把它当成返回原 World。所有上下文是调用方借用的时间／VDP／pin／mask／阶段，音频存档未复制这些 owner 或保留操作历史。

## 原生证据与测试

[失败范围](FAILURES.md)、新测试及原版预期在生产实现前冻结，见 [test-first.json](test-first.json)。真实缺 API 红灯和全部准备／首次失败记录保留；Go 测试未作后续修改。

- 两个固定核心各执行 1,872 个明确私有尾部案例，覆盖 caller mask 0／3／4／7、初始 pin 0／4、H-counter 0／1／9，以及所有 CPU 相位和跨扫描线的入口。两核心时钟、refresh、全部被保存的 14 个寄存器、原返回栈、pin／mask／VDP 结果一致；额外 118 次观察器关闭／开启结果一致。
- 15 个自然案例和 8 个声明的私有整帧案例，共 23 次真实声音调用返回到原 World 的过程被动记录。2,400 个冷启动视频／RAM／PCM 原对照全部相同，私有整帧的观察器开关结果也相同。
- 1,895 个尾部案例包含 6,784 个 CPU 步骤；逐步比较时钟、VDP、pin、mask、阶段和 reload 写入。8 个案例实际续接到世界中断，明确保留为待处理分支。
- 使用真实混合器、PCM 和声音驱动验证完整调用、共享预算、返回后恢复 World、晚期预算和后续 World 失败的共同回滚。旧前置流程、完整声音 IRQ、控制器扫描、场景工作及普通 IRQ 接受继续回归。

| 范围 | 顶层测试 | 含子项记录 | 结果 |
| --- | ---: | ---: | --- |
| 原生竞态关联回归 | 19 | 977 | 通过，66.88 秒 |
| 实际 Node／Wasm 关联回归 | 19 | 977 | 通过，44.32 秒 |

两端测试名称集合相同，相关 vet、格式、链接及差异检查通过。正式命令、环境、输入和源码摘要由 validation.json 绑定。

私有尾部案例的后台反馈明确为零，并按外部声明的 active-line 顺序推进 VDP。自然案例使用观测到的外部批次延迟，不能据此声称从新建比赛已独立重现原版全部后台／世界时钟。会话集成测试另外使用实际独立混合器。

## 取证准备修正与保留记录

最初试图使用 `m68k_set_int_ack_callback`，固定核心没有导出该符号。原核心采用编译期确认函数，因此保持核心原样，改用 RTE 执行前后及最终 pin／mask／VDP 结果，并结合源码顺序验证；没有声称观测到确认函数内部的时间点。失败日志和原尝试源码保留。

首次私有取证把可执行 PC 限于采样尾部和 late-reload，随后确实遇到下一次世界中断。取证扩展为在该中断第一条操作前停止并标记待处理，没有执行或近似未计价的世界路径。

私有 stepper 手动记录 CPU 操作，初版没有安装内存 hook，漏记了真实 reload 写入。冻结的 Go 断言正确失败。仅增加被动 write16 委托重新取证，所有原非 IO 事件、时钟、寄存器和返回结果逐项保持一致；原 fixture、执行源文件及失败日志仍保留，测试和生产逻辑没有为该错误调整。见 [修正记录](private-io-correction.json)。

## 复跑与剩余工作

```sh
python3 -B scripts/yuyuhakusho/verify_battle_irq_return.py --mode native-race --out .local/md-yuyuhakusho/return-recheck-race
python3 -B scripts/yuyuhakusho/verify_battle_irq_return.py --mode wasm --out .local/md-yuyuhakusho/return-recheck-wasm
python3 -B scripts/yuyuhakusho/verify_battle_irq_return.py --mode vet --out .local/md-yuyuhakusho/return-recheck-vet
python3 -B scripts/yuyuhakusho/verify_battle_irq_return.py --audit
```

原生及导出复跑使用尚不存在的工作区目录：

```sh
python3 -B scripts/yuyuhakusho/capture_battle_irq_return.py --out .local/md-yuyuhakusho/return-recapture
python3 -B scripts/yuyuhakusho/capture_battle_irq_return_natural.py --out .local/md-yuyuhakusho/return-natural-recapture
python3 -B scripts/yuyuhakusho/extract_battle_return_fixture.py --out .local/md-yuyuhakusho/return-reexport
```

采集输出目录由命令选择；离线导出默认读取正式保存的同字节原生 fixture，也可用 --private／--natural 指定重新采集的文件。

仍须接入待处理的世界中断、延迟寄存器写 IRQ／其他降屏蔽路径、跨帧与 DMA、其他世界工作、唯一主时钟下的联合帧存档及物理对向按键桥接，再完成平台 PCM、资源发布和完整浏览器对局验收。试玩服务保持原构建，完整比赛音频仍未开启。此前不可复现的 Go/Wasm heap 异常仍未宣称修复。
