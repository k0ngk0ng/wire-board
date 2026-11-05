# 树特殊动作自然采集失败清单

本清单先于新增的 special-only 采集器。原 `capture_fighter7_attacks.py`、专属 Go 测试及其 14 份冻结来源保持原 SHA。

- 控制来源不一致：必须使用 Root 冻结、无 observer 的唯一 5502 帧 original control，入口截止 5500，另保留 2 帧排空。输入和参考帧 SHA 必须匹配对应 manifest。
- 观察器改变执行：hook core 冷启动完整运行 5502 帧，每帧 RAM／视频都须相同；不写游戏 RAM、寄存器、周期或中间状态，不运行第二份 original control。
- 根命中被误当作成功：14 个玩家可达根分别按 selector 模式、原请求、按钮优先级、构造入口、动作／姿态、请求清零及实际能量高 word 扣费核对；低 word 原样保留。
- 空间与方向语义错误：R1 按 Facing.bit0 定向 VX，并只清 Aux48 最高 byte；R2 需主子槽空，只写 VX 高 word；地空 R3 仅写 Aux48 最高 byte；空中 R2 由 Held 更新 Facing。完整字节结果另交给已冻结的 Go 测试验证。
- 弱生命条件缺失：R6 必须为 Health signed≤111、Held.Down、实际扣费 200。R4 无玩家命令绑定，不计自然成功。
- 调用上下文串用：P1 必须为 `8400`、source7；AI9 的独立父体仍为 `9000`。返回 SP 必须为入口 SP+4，A5 不变，入口返回地址必须为自然 selector 调用点。
- 差异被掩码忽略：沿用冻结的 CPU 64 KiB 审计，记录 24／32 位 IRQ 点、各 RTE 和 `37c56/37cac`，按事件顺序解释所有外部变化；未知差异不得通过。
- 声称过度：本采集器只观察 selector 完整返回。三个子回调、回血、继续动作和完整世界由对应范围独立验收；不能仅凭 selector 根成功扩大这些结论。
- 产物不可重放：保留压缩完整调用、SHA manifest、14 根成功明细和固定命令；结束后仅清理本范围重复副本与 JSONL 中间文件。
