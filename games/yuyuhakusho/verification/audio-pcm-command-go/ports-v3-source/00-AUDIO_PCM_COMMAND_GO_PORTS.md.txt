# 原时刻 YM 地址与数据端口输出

`Config.IncludeYMAddresses=true` 已通过新 23 组原版输入。独立设备产生的 519,256 个 address／data／bank／reset 事件按实际执行序与 raw cycle 逐项相同；1,274 个参考 frame 的原 YM 地址 latch、状态反馈和 bus 事务也相同。

旧 v1、控制 v2 和 Bus 的 data-domain 向量及断言保持不动。默认视图不新增地址事件，完整地址视图通过 immutable Config 与 checkpoint identity 区分。两个视图使用同一个状态机和 pending 输出 owner。

## 实际写点

新增 `YMAddress` 事件带真实 port0／2 和寄存器 byte；DAC、pan、enable 和 YM data 事件的 `Port` 为真实 data port1／3。完整 adapter 按 ledger 顺序直接写同一芯片，不能把地址挪到 data cycle。

地址在现有语义阶段的实际 write part 输出。single 地址至 data 的间隔为 17 T-state（255 master cycles），dual 为 11 T-state（165 master cycles）。初始化、自然结束与 reset boot 的 B6／28／2B 地址也使用各自原时点。PCM 设备没有新增寄存器 latch 或第二芯片状态。

## 原失败与中途状态

新原场景 `single-reset-between-address-data` 在 2A 地址已写、DAC data 尚未写时 assert reset，并持有到后续 frame。原 frame0／1 的实际 YM latch 均为 2A；旧 data-time 补地址只留下此前的 2B。

Go 先对旧 data-domain 视图执行同一原输入，实际取得 `native2A / actual2B` red，保存失败测试与日志。23 组 original-only port／latch 向量和新 API E2E 随后冻结，缺少 `IncludeYMAddresses/YMAddress/Port` 的实际编译 red 先于实现。没有改写旧向量或移动 raw cycle。

新 E2E 用实际 YM 端口实现接收真实写序，并逐 frame 对照原 latch。reset 的 latch 投影按原 warm-reset 行为保留现有 address；该项不声称完整 YM／FIR reset 状态也已由这项投影证明。

## 状态与消费者

输出 view 参与不可变配置身份；包含真实地址的 pending ledger 不能以旧 data view 恢复。pending 地址事件只允许 port0／2，data 端口与事件含义一致。队列仍有界且只保存未消费事件。

分段 Bus 的 `DrainLedger` 会及时交付配置所选视图。完整芯片与 YM 主服务的共同 latch 验收使用真实端口视图，旧 data-domain 服务证明仍按原边界保留。世界／IRQ 调度、完整混合音序与平台播放由上层继续组合。
