# 真实 YM 服务总线桥的先行失败场景

本清单和下面的新 E2E 先于分段桥实现。v1／v2 的原向量、断言与源归档保持不动。

- 由整 frame 的固定表／回放结果代替真实设备的阶段、总线 ACK 或 mailbox；预跑 origin 后把早前 bank steal 再计入本次 assert。
- 首次 assert 的辅助预算被改成偷周期后的 main cursor；主时钟 owner 被复制到设备，或 Device 强行把原子过冲 frontier 当成 main 返回值。
- at 略早于已完成的辅助 frontier 时拒绝或继续执行；重复 assert 增加引用计数；未持有时 release 仍对齐／推进。
- Mailbox 读取另加一遍已经由 ServeYM 计入的 7 master cycles，或返回伪造的忙值。
- open bus hold 跨 frame 后丢失真实阶段；在 frame 未就绪时 flush，或 RunUntil／bus 操作偷偷 flush。
- 尚未消费的真实输出丢失、重播、变成无界历史；快照／Drain 共享可变 slice 或 bus-effect 指针；原子失败时只提交部分源状态或输出。
- 在 held 外发布 descriptor、坏资源、bad DMA／超限 pending 或时刻溢出被接受。主服务与芯片都必须及时收到同一 ordered 输出，不能等帧末补播。
- 缺少 YM address port 的 data-domain proof 被扩大成完整共享 latch／每步 pin-edge 证明。

桥从 empty typed command 初始化，先真实推进至指定服务 origin，再调用服务。不使用预初始化原 mailbox 或未来 DAC 时间表。初始桥先对原非 reset 命令／窗口逐 frame 核对 ledger、grant/resume、状态和输出；成功 ServeYM leaf 用另一个独立原 native oracle 补充。
