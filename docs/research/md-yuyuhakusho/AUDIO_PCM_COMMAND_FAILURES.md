# PCM 命令生命周期的先行失败场景

本清单先于新命令层、观察器、连续命令模型与测试。上一阶段 `audio/pcmplayer` 及其原夹具和验收记录保持冻结。本组生产范围为新 `audio/pcmcommands`；世界、仲裁 helper、公共音频与进度索引保持只读。

- 从原已启动初态开始却宣称命令启动已恢复；遗漏 idle 轮询、single 128-byte 预装、双槽首次转交与解码启用之间的阶段。
- 普通 `380a8` 的第 7 字节（单模式 pan、双槽 routing）与音乐 `37724` 的前 6 字节加轨道 pan 混用；把 descriptor 地址／ROM 头带入运行时。
- 请求 descriptor 已发布但播放器尚未锁存、已锁存 rate 但源字段尚未读取的状态错误；一次描述更新影响了不应受影响的旧阶段，或原缓冲尾部被新请求提前清空。
- 正在运行的 single／dual 模式互换、同槽替换、另一槽追加、重复资源的新请求、忙标记和 mode/slot flags 传播错误。
- 新层复制普通优先级仲裁或 score 音乐仲裁，产生两个 state owner；把待发布状态和已确认模式当作同一个 byte。
- 把 `02/03/04`、音乐 DAC track-end 或 FM／PSG 行组清理当成 PCM 立即 stop；真正硬复位与自然结束未区分。
- 把暂停当成清空播放器、补播全部过期样本或继续消耗源字节。原暂停保持 Z80 bus；释放由固定 core 对齐至 `ceil(release/15)*15`，不能简单对旧过冲 cycle 取 max。
- 只在整字节结束后统一追加 bus hold／DMA 时间，遗漏中间 bank 读取和高／低 DAC 位置；把某次原 bank 时刻或 PC 路径作为未来输入。
- 原 `z80_run(target)` 在原指令边界结束的过冲没有由有界 typed 语义阶段表达；任意 PC／通用寄存器／指令解释器被伪装成独立播放器。
- 对 busy／中断屏蔽／YM 写入临界区的权限错误；发布 descriptor 时 Z80 未实际暂停，或 release 后原 staged 数据可见性错误。
- bank 读取忽略 `phaseclock.ResolveBankAccess` 的 DMA 前推、70/77 参考 68k 延长和等待标记；用全局余数取代当前 frame 相位。
- 窗口跨 frame、恰边界、重复窗口、重叠／乱序命令、时刻溢出、坏资源、错误协议、无界积压和恢复状态非法时部分提交。
- 原生观察器改变程序、参数、返回值、bus 语义或 PCM；只比较快照／ACK，未比较连续原事件、播放器状态、样本数量与 PCM。
- 一个私有命令可达场景被扩大成任意世界 producer／IRQ 调度、完整 FM／PSG 或真实扬声器输出已完成。

首个真实 red 为缺少该独立命令层。正式 Go API 测试在新实现前冻结。先从 empty/idle 状态采普通 single、dual、一槽追加和自然结束，再覆盖替换、模式切换、音乐协议与 bus-held pause/resume。每个环境窗口均为明确 typed 输入，不由原未来 PC／时间表导出生产调度。

扩展原生组另观察到一个必须保留的参考核边界：`single-reset-during-dac-operation` 的总线申请令原 DAC store 完成于 7,186,425，随后 68k 的 `fm_reset` 仍带原请求时刻 7,186,321。事件调用顺序与 raw cycle 因而相差 104 周期并逆序。后续 DSP 研究须保持调用顺序和 raw cycle；不能排序、clamp 或倒退已推进的芯片 tick。现已冻结的 DAC-only Queue 单调输入合同不自动覆盖该域。

完整芯片接线须导出真实 YM address port 写。现有 data-only ledger 在 dataCycle 临时补地址可覆盖当前 DAC PCM，却不能证明保留 address latch 的 warm reset 在任意半帧也正确。新增原生组在首个 single 地址2A已写、data尚未写时 assert reset 并跨帧持有；旧向量不变，新观察器分别记录 address 和 data。设备应在已有语义地址写点发事件，不持久另一份芯片地址影子。
