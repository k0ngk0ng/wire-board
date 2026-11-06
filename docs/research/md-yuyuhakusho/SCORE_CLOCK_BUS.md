# 真实 PCM 设备桥与 YM 服务的成功重试

独立 PCM 设备从 empty 发布普通 `73` 后，与 `ServeYM`、真实 `chips.Interactive` 的组合已通过四个原 native 场景。71 个 Main bus／mailbox／chip 访问、475 个辅助语义事件、7 次实际忙 mailbox 读取，以及最终 main clock／refresh、辅助 frontier 与 Status 均相同。每次总线请求后还通过 JSON 保存／恢复持有态和未消费输出，继续得到相同结果。

这项证明补齐先前「成功 mailbox 重试没有接独立播放器」的局部缺口。给定的 main service origin 仍是条件输入；世界到该入口的 CPU／IRQ 调度、辅助 raw address 的中途锁存、完整 mixed PCM 和平台声音接线仍未由本项证明。

## 四个真实原场景

原工具复用已经冻结的 empty-player 初始化和普通描述发布协议：`New/idle → assert(0) → publish73 → release(0)`。辅助 CPU 真实执行到给定 main origin，然后原 Main CPU 在实际寄存器服务入口执行。没有借已播放的 PCM 状态，也没有用原期望表作为 mailbox 返回值。

| 场景 | bank | main origin | 实际非零 mailbox 读取 | main 结束 |
| --- | ---: | ---: | ---: | ---: |
| ready-bank0 | 0 | 50,000 | 0 | 52,072 |
| ready-bank1 | 1 | 50,000 | 0 | 51,932 |
| critical64-bank0 | 0 | 70,933 | 1 | 74,377 |
| startup-critical128-bank1 | 1 | 63,273 | 6 | 73,864 |

全部四例在两份原 core、双 recorder 开／关下，CPU 原步骤、完整 serialized state、refresh、RAM 和 ZRAM 相同，共 16 次原服务。辅助 PCM observer 与 Main port observer 逆序恢复，未覆盖彼此 handler。原始 bus／mailbox 时刻见[边界报告](../../../games/yuyuhakusho/verification/score-clock-bus/native-boundaries.json)。

## 为什么必须查询真正 Busy

critical64 场景给定入口 `70933`，按已独立恢复的固定前缀，第一次实际 assert 为 `71535`。此时 DAC 已完成一次写入，但 `1f2a` 仍为 64。Main 在 `71920` 读取 mailbox，值仍为 64，尽管未持有情况下的清除时刻本应是 `71640`。

持有总线使辅助状态停止推进，所以不能按 main/wall time 推算它已经清除。实际释放于 `72088`，辅助 cursor 直接对齐到 `72090`，之后真正运行辅助语义，第二次读取才得到 0。新 E2E 的返回值全部来自真实 `pcmcommands.Device.Mailbox`，没有 fake callback 或原数据查表。

给定入口之前的普通音效启动包含 128-byte 预取和 bank 占用。E2E 先用真正 `RunUntil(origin)` 推进并核对其 frontier／历史 steal，再采用声明的 main origin；后续 `RequestBus` 只将本次新增 bank steal 加入其返回游标。没有把启动期的历史占用重复算进本次服务。

## 独立组件与唯一 owner

E2E 使用真实的 `New → RequestBus → PublishAccepted → RunUntil → DrainLedger`，直接经过生产 Device 的状态机、资源读取、解码、总线效果和当前 Busy。每次 assert／release 都只有一个硬件 level，重复 assert 不再次执行或增加 refcount。release 直接 ceil15，不与旧游标取 max。

辅助未消费输出由 Device 的单一有界 `PendingLedger` 维护。测试在 request 后进行 JSON snapshot／restore，再 transfer-and-clear 到消费者。芯片 busy 仍只在 `chips.Interactive` 中，服务在原位置即时查询；测试不保存 busy deadline 或寄存器影子，也不等帧末才应用已经产生的声音语义事件。

新文件只有 E2E 和证据，没有新增默认生产 adapter。本轮 test harness 将 DAC／pan／enable 等 data-time 语义事件展开为同 cycle 的寄存器地址／数据，目的仅是核对该已保护的 Main 服务与真实反馈。这种展开不等于原辅助地址边沿：当前 Device 的 data-view 未独立发出地址事件，不能声称中途 shared latch／完整芯片状态与原版每步一致。后续 RawPorts revision 必须按真实 address 和 data 时刻分别验证，不能用本轮结果替代。

## 测试顺序与限制

[失败清单](SCORE_CLOCK_BUS_FAILURES.md)先于组合采集。Device bridge 自身已有先行缺 API red 和 17 个真实原窗口 E2E；本轮新增的是组件已经存在之后的完整组合验证，没有新增单元实现后补测或伪造 missing-symbol red。E2E 测试文本先冻结，通过 overlay 执行后，再以相同字节写入正式测试，期间没有修改 production 来迎合期望。

native race／Wasm 各一个组合 E2E、四个完整子场景和 vet 通过。native 四场景另以新进程重采，完整压缩数据逐字节相同。数据和全部 SHA 见[验收索引](../../../games/yuyuhakusho/verification/score-clock-bus/validation.json)。

给定 main origin 不意味着 0→origin 的 preceding Main 执行已还原。原生测试使用声明的无额外 IRQ 服务初态；辅助 raw address／data 间的锁存、硬复位和更完整时序，由对应下一 revision 独立验收。没有实际平台播放、扬声器等价或全球 world→IRQ 调度完成声明。
