# PSG 叶操作的先行失败场景

本清单先于新 PSG 实现和测试编写。范围是 scoreclock 中固定语义的 PSG raw store 与叶操作；冻结的 YM 服务、score、chips、phaseclock、engine 和总索引不在改动范围内。

## 原版行为与待验证项

- 原版 `vdp_write_byte` 直接调用 `psg_write(m68k.cycles, data)`。错误地复用 YM／ZRAM 的 7 master cycles 延迟，会移动全部 PSG 写点。
- 单写与音高双写使用 data-register 到 absolute PSG 端口的固定 store。每次 store 的 refresh 检查必须位于写之前；不能将全部 refresh 统一累加到末尾。
- 音高双写必须保持 latch 在前、6 bit continuation 在后；两个写点不能合并。
- 静音叶操作包括选择端口、按 `9f/bf/df/ff` 顺序写四个声道，以及返回。把四次写当作相同的 absolute store 会产生错误成本。
- 叶操作从已准备好 canonical 字节的 store 入口开始。音高、音量、keyoff 的准备分支以及 caller 的调用与后续成本均不自动补入。
- raw store 只包含一次 absolute store，不包括返回；single 叶操作包含 store 和一次返回。两者不得混用或重复补返回。
- 返回也有自己的 refresh 检查。漏掉返回成本或重复补调用成本，都会错误移动下一操作。

## 无效输入与有界停止

1. 未知 kind、single 带 continuation、音高 pair 的无效 latch／大于 63 的 continuation、mute-all 带任意 payload，应在外部 IO 前返回错误。
2. 缺少 PSG owner、超过 65,536 的操作预算、明显溢出的起始 cycle，应在外部 IO 前失败，并返回零值 clock。
3. 预算小于完整叶操作时，应在固定语义操作边界停止，返回 `ErrBudget`；此前已发生的外部写入保持可见，不能宣称回滚。
4. 外部 `WritePSG` 返回错误时，应原样传播，停止后续写入，不能额外修改 chip state 或吞掉错误。
5. 外部 chip 拒绝倒退时间、超前或跨帧写入时，应返回该错误；本服务不得暗中 flush、改写时间或建立第二份 PSG 状态。
6. 后续固定语义操作遇到 refresh deadline 溢出时，应停止，不能发生 uint64 回绕或继续写入。

## 原生证明的边界

- 两份固定 native core 都从声明的 quiet private fixture 开始，执行原版 leaf 指令。PSG 在每次恢复 quiet world 后明确 reset；这不是自然 driver 状态。每例比较 recorder 关闭／开启时的完整 serialized state、RAM、refresh、原执行边界和 PSG context。
- recorder 必须只包装真实 PSG 写入路径，且不得改变 main cycle；原 PC、逐指令步骤和源架构成本只保留在研究产物中。
- 每种 leaf 覆盖无 refresh、入口 refresh、第二写点附近和返回附近的 deadline。两种 single 来源分别覆盖 volume 与 keyoff 的原 store。
- Go 与 Wasm 使用真实 `chips.Interactive`，比较有序端口写点、最终 clock 与 PSG 直接写入后的完整 owner 状态。
- 测试产物应绑定 ROM、两份 core、脚本、测试源码与实现的 SHA-256，并可用固定命令复跑。
- 本证明不覆盖自然 driver 调用路径、caller 成本、异步 PCM、实体硬件、产品播放或完整 driver tick 时序。
