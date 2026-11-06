# 普通 IRQ 接受与场景内部截止点

独立规则现在能在正常执行批次入口判定战斗 profile 的 4／6 级 IRQ，并计算带 CPU 相位的异常入口时间。它已与实际默认场景工作边界、现有 VDP 计数器组合验证，可以在无声音请求的场景内部找到与原版相同的普通 IRQ 入口。

`AcceptWorldIRQ` 只借用当前主时钟、请求级别和屏蔽值，返回临时判定结果。请求必须严格高于当前 mask。入口耗时在 CPU 相位为零时为 50 个 CPU 周期，其余相位为 `60 - phase`，其中 `phase = floor(master_cycle/7) mod 10`；保留 master cycle 的余数。异常入口不执行普通指令的 bus refresh，原 refresh owner 不变。

函数不保存原 CPU、SR／PC、异常栈、VDP 副本或操作历史。实际 VDP 确认和 RAM IRQ 处理继续由外层执行；它没有把声音 driver 直接安排在异常入口。

## 原版证据与先行测试

[失败清单](FAILURES.md)、原版取证、源码成本、fixture 和测试在生产实现前冻结，见 [test-first.json](test-first.json)。真实缺 API 编译失败见 `missing-api-red.jsonl`。新测试与原生预期未作后续修订。

两个固定参考核心各先核对 2,400 个冷启动视频步，再分别运行：

- 1,680 个普通入口案例：mask 0..7、请求 0／4／6、master cycle 0..69，覆盖所有 CPU 相位及不足一个 CPU 周期的余数；700 个接受，980 个拒绝。实际异常栈的返回 PC／SR 同时核对，两核心完全一致。
- 32 个完整默认场景计算的中断截断案例：初始化／运行、Pan 0／1、0／4 个摄像机样本及四个入口时间。原核心完整执行到外部给定的首个 HINT deadline，再接受真实 4 级中断；记录实际被打断的工作边界和异常入口时间，两核心完全一致。

Go 对应测试使用真实 `StepChuWorldObjectsWithHooks` 与现有 VDP 计数器。初始计数器为声明的 9，首个有效 HINT 为第 47 条帧内扫描线的 160,740 master cycles。给定同样的回调入口与 VDP 上下文，32 个工作截止点、refresh 与异常入口均逐项匹配原核心。停止介入后，世界事务返回原状态且不泄漏局部事件。

原生竞态与实际 Node／Wasm 联合回归分别通过 7 项顶层测试、802 条含子项记录，耗时 11.86／11.43 秒；其中复用完整 792 个默认场景／摄像机调用的 187,824 步成本对照、内部真实音频和回滚测试。相关 vet、格式与差异检查通过。记录及 SHA-256 绑定见 validation.json。

复跑命令使用工作区内尚不存在的输出目录：

```sh
python3 -B scripts/yuyuhakusho/verify_world_irq_acceptance.py --mode native-race --out .local/md-yuyuhakusho/irq-recheck-race
python3 -B scripts/yuyuhakusho/verify_world_irq_acceptance.py --mode wasm --out .local/md-yuyuhakusho/irq-recheck-wasm
python3 -B scripts/yuyuhakusho/verify_world_irq_acceptance.py --mode vet --out .local/md-yuyuhakusho/irq-recheck-vet
python3 -B scripts/yuyuhakusho/verify_world_irq_acceptance.py --audit
python3 -B scripts/yuyuhakusho/capture_world_irq_acceptance.py --out .local/md-yuyuhakusho/irq-recapture
```

场景成本的完整回归和范围见[默认场景工作验证](../world-arena-work/README.md)。增加普通 IRQ 规则没有修改其既有生产路径、测试或原版夹具；更早的源码绑定清单按历史保留，不改写原 SHA。

## 仍需接通

本次只处理普通执行批次入口。延迟寄存器写入 IRQ、降屏蔽指令内部接受、RTE、RAM IRQ 的等待／控制器扫描／声音前置流程，以及其他世界工作尚未接通。

arena 回调入口和初始 VDP 上下文均是给定输入，不能据此声称已从新建比赛独立生成整局 IRQ 时间。必须继续计价人物、碰撞、其他场景、入口及尾部工作，形成唯一的主时钟和联合帧存档，之后接通完整比赛音频。当前试玩服务仍使用原构建，完整比赛音频未开启。
