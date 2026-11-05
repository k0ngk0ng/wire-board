# 单次声音服务时序的先行失败场景

本清单先于 `audio/scoreclock` 实现。本轮先恢复给定 canonical YM 请求入口、当前参考帧时刻、refresh deadline 与外部 bus／mailbox／chip 状态后的内部握手；不把世界到 IRQ、完整音序解析成本或既有意图序号当作已完成赋时。

- 将整次 YM 写固定为一个成本，遗漏寄存器范围 `40..4f` 的 cache 路径、主／次 bank 分支和调用入口差别。
- 在读取 busy 之前就写地址／数据，或同一 cycle 重排地址和数据；status 的低位定时器标记被误当 busy。
- 忽略 Z80 bus assertion、BUSACK 的 read-modify-write、`1f2a` mailbox 以及释放／16 NOP／重试路径。
- 混淆请求 Z80 执行到 target 与 release 时直接 ceil15 重设；把先前的过冲 max 合并到 release。
- 忽略每次 68k 访问 ZRAM／YM 额外的 7 master cycle，或在错误位置施加该延迟。
- 将 refresh 统一加到函数末尾；跨越 refresh deadline 的原语义操作必须保持检查先后，不能用平均常数拟合。
- 复制 YM busy deadline、Z80／68000 时钟或 mailbox 状态，形成第二份 owner；status 必须读即时 chip owner。
- 只核对最终数据，未核对每次 poll、BUSREQ、地址／数据写的实际顺序和时刻，或 observer 改变原状态。
- 用原 PC、原指令流／opcode、逐 tick 绝对成本表、原端口时间表或 CPU／RAM VM 实现运行时。
- 用已消除 F6／F7／F8／F9 控制路径的 before／after 或扁平意图反推全部原解析成本。
- 私有初态被写成自然执行覆盖，或把给定入口／外部反馈后的局部服务通过写成自主世界／IRQ 调度。
- 错误输入或超过有界轮询预算时仍静默无限循环；跨帧时未经调用者明确 flush 便改写芯片状态。

先取得无时间意图不足以通过赋时契约的真实 red；Go 公共接口及 native fixture 检查必须先于实现。两份参考 core、旧证据、冻结 `phaseclock`、`score`、`chips` 与世界引擎保持原样。
