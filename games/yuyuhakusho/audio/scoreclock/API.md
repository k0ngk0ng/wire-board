# Canonical YM 服务时序

`ServeYM` 从给定的 canonical YM 请求入口和 `Clock` 开始，按固定参考语义成本调用外部 bus／chip owner。它不选择 driver tick 起点，不计算完整音序解析成本，不执行 CPU、ROM、opcode 或原 PC 时间表。

```go
result, err := scoreclock.ServeYM(
    scoreclock.Clock{Cycle: origin, Refresh: refreshDeadline},
    scoreclock.Request{Bank: bank, Register: register, Value: value},
    busOwner, chipOwner,
    scoreclock.Limits{},
)
```

`Clock` 只作为本次调用的输入／输出游标，不在包中持久保存。调用者在成功后采用 `result.Clock`，不保留第二套全局时钟。`Refresh` 是参考 main-CPU 的独立 refresh deadline。相位到达 deadline 时，当前固定语义操作先设置下个 deadline 为当前相位加 896，再付出 14 master cycles；每次 YM／ZRAM 访问另付出 7 master cycles。

| 接口 | 时序与状态归属 |
| --- | --- |
| `Bus.RequestBus(at, asserted)` | 外部辅助播放器／总线 owner 处理申请或释放；返回真实同步后的 main cursor，必须不早于 `at`。 |
| `Bus.BusGranted(at)` | 在 read-modify-write 的原位置读 BUSACK，随后本服务再次申请；不缓存授权状态。 |
| `Bus.Mailbox(at)` | 在包含 7 master cycles 访问延迟后读当前 mailbox；非零则释放、执行固定 retry delay，再申请。 |
| `Chip.ReadStatus(at, 0)` | 直接读取即时 busy。只测试 bit7，chip owner 保留 timer／busy 的唯一状态。 |
| `Chip.WritePort(at, port, value)` | 先地址、后数据，同 cycle 顺序不变；本服务不持有芯片寄存器副本。 |
| `Result.Level` | 报告 `40..4f` 原缓存写点的临时语义 effect；不建立可写 RAM 或第二份 level cache。 |

`Level.Value` 是本次提供的 `Request.Value`。score 的 chip-effective 投影可能已删除硬件忽略位，不能拿这个字段重建原 TL RAM 的高位；typed score／patch 仍保持唯一参数归属。

默认最多 4,096 个固定服务操作，显式上限为 65,536。超过操作预算返回 `ErrBudget`。未知 bank、非法上限、初始游标溢出等输入在外部 IO 前拒绝。请求 bus 的外部错误、回退的 cursor 或 chip 错误原样失败。

调用者必须在 detached transactional owners 上执行，成功后再提交。失败可能发生在此前已有外部 bus／chip 操作之后，本包不暗中复制或回滚这些 owner。跨帧 flush 由调用者明确执行；不能靠预看 busy deadline 或移动旧事件隐藏等待。

当前原生证明是「bus 已持有、mailbox 为 0、服务尾部不插入 IRQ」的 48 个声明私有状态：两 bank、三个寄存器范围、四种起点／refresh 相位和两种初始 busy。472 个有序访问、16 个 cache effect、最终游标／deadline 与两份 native core 相同，Go 直接使用真实 `chips.Interactive`，共实际经历 40 次 busy poll。未就绪 bus／mailbox 的有界停止属于先行测试，尚不是其完整原生成功路径证明。

完整 driver 操作标签与原自然 pause／priority 边界的要求见 [SCORE_CLOCK.md](../../../../docs/research/md-yuyuhakusho/SCORE_CLOCK.md)。
