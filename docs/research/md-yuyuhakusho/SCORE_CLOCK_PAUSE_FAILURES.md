# 暂停与恢复时基的先行失败场景

本项恢复 gate 之后固定暂停／恢复 caller，组合已验证的实际 YM／PSG 服务及 bus／chip owner。使用已有自然对局 IO 与静态成本，不启动新的原生采集或硬件诊断分支。实现前覆盖：

- 将保存 SR 的 14 CPU cycles、实际 PC 相对 JSR 的 18 cycles、八次 long 保存／恢复或 DBF 末轮成本算错；误按 absolute JSR 的 20 cycles 处理，或将 refresh 统一放到末尾、漏掉普通总线同步造成的时钟前进。
- 申请／释放不是同一硬件 bus，跳过 BUSACK、mailbox retry 或即时 YM Busy 读取，或在 helper 返回时擅自保持整帧暂停 bus。
- 捕获发生在静音之后、未从实际 chip 读取 24 个有效 TL，或原字节被硬件忽略的 bit7 当作有效音量。保留原 raw 值，明确只对有效 TL 投影比较。
- 暂停写错 bank／operator／channel 顺序，遗漏四路 PSG 静音；恢复错误重启 PSG 或返回前丢失待还原音量。
- 同一操作预算被每个嵌套服务重置，使 Busy／mailbox 重试失去整个调用的上限；溢出、缺少 owner、无效 level 或错误在已有外部效果后未传播。
- 外部错误时返回半个结果；调用方未丢弃 detached owner，或把原始 canonical owner 改成半静音。
- 为共享预算重构既有 YM／PSG 服务后，旧原版固定 leaf、总线、gate、header、fade 测试变化。

自然 IO 夹具提供原版的外部 bus／mailbox／status 反馈，验证 fixed caller 与服务组合在全部已观察 Main 点的时刻及有效端口值；它不证明独立辅助设备／芯片反馈。另用实际 mixed Runtime 验证暂停、JSON 恢复、继续和失败分支的状态效果。自然记录没有 PSG 尾部和最终 RTS 的直接捕获，该尾部依赖既有 PSG leaf 原生验证和静态固定成本，不能称为新增整场逐采样验收。
