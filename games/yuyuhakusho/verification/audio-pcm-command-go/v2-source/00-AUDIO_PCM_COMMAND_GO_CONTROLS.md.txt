# PCM 命令 Go 控制域

新增控制层已在 22 组原版输入上通过。expanded20 的 1,202 个参考帧、312,410 个有序 data／control／reset／bank 事件逐项相等；两个独立补充场景覆盖硬复位后的双槽自然结束，以及 reset raw cycle 跨过 chip tick 的逆序。所有 frame 都先 JSON 保存恢复自己的状态，再继续推进。

原生竞态和 Wasm 各 9 项顶层检查、50 个子场景通过，vet 通过。v1 的 12 组原向量和断言保持不动，历史源独立归档。

## 控制与事件

`SetRestorePan` 更新实际以后自然结束时使用的恢复 byte，不立即修改当前 DAC 声像。完整 B6 byte 的低位保留。`HardReset` 在 assert 与 release 各输出一次实际 reset，release 对齐至 15 master cycles，然后执行固定 typed boot：pan0、YM28=6、enable0 和相应 global／slot 初始化按原次序发生。

`Frame.Ledger` 是完整的当前 data／control／reset 观察域，按实际调用顺序返回。`Frame.Events` 从该 ledger 派生旧六类兼容视图，二者都只是返回值，不建立持久事件副本。当前还未记录 YM address port，不能把本证明扩大成完整 `WritePort` 总线。

复位 raw cycle 可以回到先前 DAC raw cycle 之前。独立 `single-reset-crossing-chip-tick` 原版例中，已开始的 DAC 操作先完成，然后 reset handler 才收到较早的原请求时间。设备保留原执行序；不排序、对齐或 clamp 以制造相等。

## 资源与 DMA

`7F` 的 1-byte 私有 sample 仍进行 128-byte 预装。127 次资源外读取只发出 bank 时序事件，设备不调用 `ByteAt` 读取资源外字节，也不填入邻 ROM 数据。该私有合同与公开 finite 目录的上下架范围分开。

`F0` 使用真实 descriptor index122 的黑龙资源，按 encoded SHA-256 定位。它不映射到 `7A`。

每次 bank 读在自己的语义发生处调用统一 `phaseclock.ResolveBankAccess`，完整比较参考的访问前相位、DMA 后相位、辅助恢复时刻和 68000 延长。当前设备拒绝 `Remaining != 0`；这类 wait flag 需要外部 VDP 的分段停止／清除调度，helper 算术相等不能证明该环境已实现。独立真实接受 red 先于该拒绝修复。

## 验证范围

expanded20 的 17 个非 reset 场景有 802,191 个立体声 PCM 帧及每帧完整 DAC DSP 状态相等。reset 场景按完整 ordered 事件、状态与总线事务验收；旧 DAC Queue 的 raw-time 单调合同没有放宽，也没有修改旧 DSP。父研究另保留 ordered reset bridge 与错误排序的原 PCM 反例，不能将其称为完整 FM／PSG reset 已接通。

初始 20 组中的 `dual-hard-reset` 只是一段有效继续流；独立 55-frame `dual-hard-reset-complete` 补充证明了后续自然结束。原 bounded 向量未被延长或覆盖。

`Hold` 是明确的硬件总线窗口。自然游戏暂停还包含世界／IRQ producer 的安排，本包没有把私有 transport hold 窗口扩大成完整 GamePause 调度。

先行失败、原生源和冻结索引位于 `games/yuyuhakusho/verification/audio-pcm-command-go/`。后续真实 YM 服务需要分段推进同一设备、读取实际 mailbox，并让生成的 chip 事件及时进入同一合成 owner；静态环境表不能代替这一连接。
