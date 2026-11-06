# 真实 PCM 总线与 YM 服务桥

Go 设备现可在一个参考 frame 内分段推进、申请或释放同一硬件 bus、读取真实 mailbox，并及时交付真实输出。17 组原版非 reset 命令／窗口通过新接口完整执行，272,526 个原事件、native mailbox、grant／resume 和 frame 状态逐项相等。原生竞态与 Wasm 各 13 项顶层检查、67 个子场景通过，vet 通过。

新接口复用同一个 typed stepper。它不复制主时钟，不保存输入回放历史，也不从未来 PC／DAC 时间表决定状态。

## 时钟和硬件持有

`RunUntil(at)` 的预算始终是调用者最初给出的 at。该段实际 bank 读的 70／77 master-cycle 延长作为 effect 返回；主时钟 owner 自行接收该 effect。已在 pending 队列的旧 effect 不重复加入。

首次 `RequestBus(at,true)` 真实推进设备后置同一个 `Held.Open` level。重复 assert 无执行和引用计数。at 略早于上一次已完成的原子 frontier 时，不重新执行、不拒绝，也不把 frontier 强塞成 main 返回值。release 直接对齐 ceil15；未持有时 release 没有效果。

`Mailbox` 只返回真实 `Global.Busy`。ServeYM 已经在调用前计入其 7 master-cycle 访问成本，设备不再次加时。

## 输出和恢复

`PendingLedger` 是唯一有界、尚未消费的输出 owner。它保存最多 4,096 笔事件，供调用者在每次真正 Request／RunUntil 后 Drain 到同一芯片。`DrainLedger` 转移并清空，不保留已消费历史。快照深拷贝队列与 bus effect。

`Frame` 表示已经提交的 reference flush 次数，`Time` 表示真正执行 frontier；二者在 held 或 partial frame 下有独立含义。`FlushFrame` 只提交一个已经就绪的边界，不隐式执行 CPU。最初沿用整 frame 快照的 frontier 上限，导致所有 partial-frame 恢复被拒绝；新增原 E2E 真实失败先保留，再扩展为同一当前 frame 的合法 frontier 范围。

复核还发现 pending effect 的 uint32 resume 可被溢出伪造，独立负向先取得实际接受 red，再增加参考 frame 上限检查。原始失败和修复前源保留。

## 真实服务与证明边界

独立 scoreclock 分工另从 empty typed pub073@0 初始化，真实推进到指定 main 服务 origin，然后使用实际 Device 与芯片状态执行原生成功 YM leaf。该证据比较 Main IO、真实 mailbox、aux data 事件和最终时间／状态，具体索引由 `verification/score-clock-bus` 维护。

当前设备 ledger 是 data／control／reset 域；YM address port 的原时刻和中途共享 latch 仍由下一次独立原向量验收。不能把 data 时刻补 address 的测试 adapter 扩大成完整 pin-edge 等价。

资源读取持续借用唯一不可变 payload catalog；初始化、当前播放阶段和未消费输出都只有各自的 typed owner。世界 producer／IRQ 调度、平台音频接线与完整 GamePause 仍在上层整合范围。
