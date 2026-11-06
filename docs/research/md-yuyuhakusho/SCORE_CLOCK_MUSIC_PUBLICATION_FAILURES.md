# 音乐 DAC 发布的先行失败场景

- 把 request marker 的非零判断用于阻塞；音乐发布只检查 bit7，marker 1 仍允许仲裁。把 ModeFlags 的低位误算为优先级，或在 blocked 时额外读取 priority。
- 套用 YM busy mailbox 等待；该路径只申请总线并等待 BUSACK，busy 非零不得阻塞。
- 在反馈回调中提前发布采样，或在被拒绝时发布；实际 accepted intent 到达后才能应用，提前 Finish、错误／重复 Apply 和失败后继续均须拒绝。
- 丢失完整 track pan 的 AMS／FMS、错误使用普通音效的第七字节，或把音乐请求改成普通 single／dual 请求。
- 另建 marker、priority、descriptor、rate 或 pan owner。不可变 descriptor 只引用现有 payload，所有状态仍由 PCM Device 唯一持有。
- 8 次源元数据写入的成本、ZRAM 访问额外延迟、refresh、释放总线或最终 return 遗漏。元数据在同一 bus hold 内的整体发布必须绑定首 marker 写与末 pan 写，不能伪造源指针／RAM 写接口。
- 发布不要求实际 open bus hold、越过未 flush 帧、Begin／End 与实际完成点不符，或允许不支持的 metadata overlap。失败不得提交 detached 分支到原 owner。
- 无效 descriptor、未知 protocol、错误 rate、预算／clock overflow 或外部读写失败被吞掉。

原版 oracle 使用正式六个音乐 descriptor 的身份及 payload，声明初始 held bus、busy 9、无额外 IRQ 的 profile。Go E2E 从已完成的 DAC commit 开始，连接真实 mixed Runtime 与生产 resourcebridge。原版数据、E2E 和失败计划先于实现冻结，不修改既有 fixture 或测试。
