# 音符开关与门限

`BeginKeyTransfer` 为固定 FM／PSG key leaf 准备临时阶段。FM key-on 依次检查 note-off、claimed，key-off 依次检查 tie、claimed；PSG key-off 只检查 claimed。Begin 不提前写入，实际意图到达时 Apply，Finish 拒绝提前或重复结束。

FM 路径 tail-call 已有 canonical YM 服务，复用实际 bus／mailbox／busy 与 synthesis owner；该服务的 return 就是 key return，Finish 不再增加一次。PSG 使用实际独立端口，保留自己的单次 return。helper 不保存 gate counter、音符状态、原栈或源程序。

`BeginGateTransfer` 覆盖当前 remaining 为零、递减后仍等待，以及 remaining 为 1 时的 expiry。expiry 调用相同 key 阶段，父子及 canonical YM 共享同一 clock／refresh／预算；Expired 语义告知外层本拍声部提前返回。原 source 的父调用跳过仅计为固定成本与语义结果，不建立生产 CPU stack。

对应 effect 由 ExpectedEffect 返回，Apply 校验后执行一次。失败后需丢弃全部 detached owners；接口不负责推进 canonical remaining／note flag，这仍属于 score driver，也不自动 flush 或处理 world IRQ。

原版 288 个 profile 覆盖十种 channel、guard 组合、gate 0／1／2／255；声明初始 held bus、mailbox 0、无额外 IRQ。原栈指针只作为研究中的提前返回证明。冻结证据与复跑入口见 [score-clock-key-gate](../../verification/score-clock-key-gate/)。
