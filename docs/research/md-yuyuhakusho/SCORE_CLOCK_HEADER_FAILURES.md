# Header 固定调用边界的先行失败场景

本清单先于新 header 测试和实现。范围仅包括 control-idle、tempo、fade-idle 三个调用；每个调用包含原 caller、对应 callee 及一次返回。冻结的 YM、PSG、tick、score 和总索引均不修改。

## 静态证据与时序风险

- 原三个 caller 的 ROM opcode 均为 `4eba`，属于 PC 相对 JSR，架构成本为 18 CPU cycles。误按 absolute-long JSR 的 20 CPU cycles 计算，会让每个调用多出 14 master cycles。
- control-idle 仅支持两个控制 mode 都为 0。原路径包含两个比较、两次读取和四个长条件分支；把长分支未跳转当作短分支成本会改变时钟。
- tempo period 为 0 时直接返回；enabled 但未 reload 时递减 remaining 后短返回。两者不能合并为相同成本。
- tempo reload 会读 period 并重新装载 remaining，然后扫描恰好 10 个音乐 slot。每个 slot 先测试 active；active 和 skip 分支具有不同成本。
- active slot 的 NoteWait 增加沿用原 8 bit 环绕；计时层只消费 owner 提供的临时语义，不增加 NoteWait 或持久保存 tempo／voice 副本。
- mask 中相同数量的 active 位若位于不同 slot，可能改变 refresh 的发生边界；不能仅用 popcount 汇总全部扫描成本。
- 每次 ADDA 和 DBF 都有独立 refresh 边界。DBF 回边为 10 CPU cycles，最后一次为 14；不能统一用 dispatch 表中的 12。
- fade-idle 仅支持 FadeSteps 为 0。漏掉 MOVEQ、读取、短分支、原 caller 或返回都会让停点错误。
- 三个连续调用必须复用上一调用返回的 Clock 和 Refresh；不能每段重置 deadline、重复付返回或把 refresh 统一加到末尾。

## 无效输入与有限停止

1. 非 idle control mode 和非零 fade steps 必须显式返回 Unsupported，不能返回近似成本或静默成功。
2. disabled tempo 不得同时声明 reload 或 active mask；未 reload 的 tempo 不得携带扫描 mask；mask 大于 `0x3ff` 必须拒绝。
3. 超过 65,536 的操作预算、明显溢出的初始 cycle、后续 cycle／refresh 溢出必须返回错误和零值 clock。
4. 每种固定路径的预算耗尽必须在语义操作边界返回 `ErrBudget`；不能无限迭代或接受任意调用者提供的 CPU-cycle／opcode program。
5. 输入 work 值不得被修改；服务不得建立持久 tempo、voice、控制字段或 score 状态。

## 原生与跨平台证明

- 测试源码先冻结，再通过 Go overlay 构造缺 API red；新实现出现前保存失败日志、环境与 SHA。旧包在此期间保持可编译。
- 原生矩阵分别覆盖三个调用及连续组合，包含 disabled、step、reload，以及空 mask、全 active、全部单 active 位置和两组交替位。
- 每例在两份固定 core 上执行原指令，并比较 recorder 开关时的完整 serialized state、RAM、refresh 和逐步边界；连续组合的三个 stop 均单独核对。
- 原 PC、opcode、通用寄存器和 RAM 仅为研究 provenance；portable fixture 与生产运行时不消费原 PC／ROM／指令或逐 tick 时间表。
- Go 原生竞态与 Wasm 使用相同 fixture，核对每个停点，并保存可重复执行的命令、日志和 SHA。
- 本证明不包括非 idle duck／fade、queue、dispatch、完整 tick、世界／IRQ 排程、PCM 或产品声音播放。
