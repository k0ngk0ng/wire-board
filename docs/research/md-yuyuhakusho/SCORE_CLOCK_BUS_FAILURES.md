# 真实 PCM bus／mailbox 与 YM 服务组合的先行失败场景

本清单先于本轮组合 oracle／adapter。已冻结 `ServeYM`、PSG、tick、header 和 PCM 向量保持原样。

- 直接借已初始化播放器或原 RAM 快照；未从 empty→普通 73 发布，便声称真实命令启动后的成功重试通过。
- 在给定 main origin 之前累计的 128-byte 预取 bus steal，又在本次 `RequestBus` 重复加进 main cursor。
- 使用基于时间或原预期序列的 fake mailbox callback，忽略持 bus 时 Busy 不随 wall/main time 自动清零。
- 首次 assert 没有真正推进独立 PCM owner 到原预算，重复 assert 又重复 run／累计引用计数，或 release 对旧游标取 max。
- 丢失 DAC／pan／enable 事件，等帧末才应用，或用新的 busy deadline 副本代替唯一芯片 owner。
- Main 实际 status poll／地址／数据写与 bus／mailbox 读取顺序不相等，却只比较最终 ACK／寄存器值。
- 把 data-time 语义事件同 cycle 补 address 当作原每个 address pin-edge 已验证，掩盖半次 port pair 内的 latch 差异。
- 原多观察器 wrapper 互相覆盖、没有按逆序恢复、改变原状态／采样／CPU 步骤，或来源文件在采集时变化。
- 将给定合法 conditional service origin 写成已经恢复 0→origin 主 CPU 调度、自然完整世界／IRQ 或扬声器等价。
- 错误、资源或有界输出失败后提交部分 owner 状态，或复制 payload、PCM／chip 状态与全局时钟。

先保存真实原 native 组合，再针对实际独立 Device bridge 编写可重复 E2E。缺少 RunUntil／open hold／Drain 接口时明确留下缺口，不喂原答案或移动事件时间来制造成功。
