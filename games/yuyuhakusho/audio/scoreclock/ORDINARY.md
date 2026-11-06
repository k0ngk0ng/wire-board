# 普通 PCM 仲裁与发布

`BeginOrdinaryPublication` 从已解析 descriptor 的普通命令入口开始，连接实际状态读取、仲裁和 accepted metadata 发布。逻辑 code 通过生产 resourcebridge 解析，metadata 再与实际 PCM library 比较；不携带源索引、ROM 地址、指针、RAM 或另一份 payload。

源程序先申请总线，在同一 hold 下顺序读取 mode、A／B flags；dual 命令还读取 A／B busy。随后释放总线并恢复中断屏蔽，在该采样视图上比较优先级与 routing。只有 flexible routing 且两声部均忙时才在实际阶段读取 Root 的 choice bit；busy 64／128 都视为占用。实际接受决定仍由 `Library.PrepareOrdinary`／`engine.SelectPCMCommand` 产生，计时路径必须与它一致。

Begin 停在 accepted 分支的下一次 effect 之前。对应 dispatch 到达时调用一次 Apply：dual 分支先通过唯一 Root owner 写 choice，再计入固定 header 工作；随后重新申请总线并发布。`StartDual` 来自第一次采样的 mode，即使两次 hold 之间 PCM mode 变化也不重采。single 发布 8 个字段，dual 发布 10 个字段，启动 dual 时再加 2 个控制字段。各次 ZRAM 额外延迟、caller、refresh 和预算均按原边界计入。

在第二次 hold 且中断屏蔽的 metadata 区间内，两个 CPU 都不能观察到中间字段；运行时在末 pan 写入时整体更新 canonical PCM 请求，并绑定首末写入时刻。原有 open hold、当前未 flush 帧、metadata overlap 与原子分支验证继续适用。Finish 负责释放、恢复屏蔽、正常 non-duck 尾部和最终 return；拒绝时不发布、不写 choice。

所有阶段共享临时 clock／refresh／预算。错误或重复 Apply、提前 Finish、失败后继续均被拒绝；失败时调用方丢弃全部 detached owners。Root 仍唯一拥有 ChoiceBits，PCM 仍唯一拥有实际 mode／flags／busy／request，resourcebridge 借用唯一 encoded payload owner。

本接口支持正式 122 个普通资源的正常 non-duck profile；7F diagnostic 不在该闭包，当前 mode 的 duck bit 也明确拒绝。包含私有 callable code 的验证不证明它由正常世界 producer 产生。该接口不计费 descriptor 之前的 Root 分派、队列／世界 IRQ 或整拍流程，也不自动 flush 或启用平台整场声音。

E2E 覆盖全部资源、正式八组 command metadata 可达的 22 种仲裁分支形状、实际 mixed／Root owner、两次 hold 与完整 return。原版对照、先行测试、采集覆盖修正和复跑入口见 [score-clock-ordinary](../../verification/score-clock-ordinary/)。
