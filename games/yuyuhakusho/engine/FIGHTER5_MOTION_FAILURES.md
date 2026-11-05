# 来源 5 MAIN 运动：实现前失败场景

本清单在 `fighter5_motion.go` 与冻结测试之前建立。范围为 source5 仙水动作槽 `11/2d/2e/2f/30/31/32`，入口分别为 `1cb08/1b67a/1b6c4/1b704/1b7fa/1b8f4/1b9bc`，终点为进入共享尾段前的 `15afc`。静态依据为 `docs/research/md-yuyuhakusho/static/fighter-action-slots-focused.txt` 与 `fighter-action-constructors-focused.txt`；它们是 direct Ghidra 的有界指令证据。REA 不支持该 raw 68000 输入，不能把静态可达性称为运行证据。

- 混淆角色来源 ID 5 与十六进制动作槽；观察共享 `11/2d` 时错误收入其他来源人物。
- `11` 不能因共享实现而忽略 source5 调用上下文：八项结构表中仅 1..5 移动，0/6/7 不动；实际姿态 `0c` 只有 0..6 帧。完成 byte 等于 1 时优先恢复，空中／地面只比较 signed 高 word。不能提前改帧表。
- `2d` 完成条件是 bit0，不能改成整 byte 等于 1；只有 ticks 非零、帧至少 2 且不等于 14、A/B 新按下才清 ticks 并 byte 递增，须保留 255 回绕。
- `2e` 完成条件也是 bit0；完成时先清 completed，再站立。未完成仅帧 0 且接触 bit2 才调用组合结束构造；姿态恰为 `a4` 选 `1926a`，任何其他姿态选 `192ae`。ticks 不参与门槛，其他接触位不能误触发。
- `2f/30/32` 完成条件是整 byte 等于 1。`2f/32` 清 completed 并站立；`30` 直接调用 `1863a`，必须 byte 增加 ExtraJump、重置、从当前 immutable movement source 恢复 Gravity，分开算术移位后求 3/4 纵速，并仅面对 byte 恰等于 1 时保留正横速。不能把该分支复用为空中零速恢复。
- `2f/30/32` 仅帧 0、ticks 0、第一保留子槽 Size 为 0 时建立对象。任何非零 Size 都应原样保留三个完整子槽；未使用槽不能被清空。完成分支优先于建立门槛。
- 子对象只重写原指令写到的字段：Behavior/Size/Stage、Auxiliary76、Team 加 16 的 byte 回绕、TargetBit 加 1 回绕、属性 `((parent+0x60)&0x9fff)|0x2000`、Ground 仅高 word、速度 full long、完整 X/Y 后仅坐标高 word 偏移、Facing、Depth、Layer 减 4 byte 回绕、TargetMask、Pose、Gravity 仅最高 byte、真实 `+fe` Owner word。其余字节必须保留，尤其 Gravity 低 24 bits、Ground 小数和子槽所有图形／动画／战斗字段。
- `2f/32` 子对象 X/Y 高 word 偏移为朝向 0 时 `(+28,-64)`，任意非零朝向时 `(-28,-64)` 且 neg.l 横速；`30` 为 `(+20,-52)`／`(-20,-52)`。不得整 long 加偏移导致小数跨界。`2f/30` Behavior 为 `1414e`、Pose `17`；`32` Behavior 为 `143e4`、Pose `18`。
- `31` 每次先对唯一 `+48` long 的高 word 加 1，并保留低 word；比较为 signed word 大于 31，含 `7fff→8000`、`ffff→0000` 回绕。不能把计数当 byte 或 unsigned word。
- `31` 计数不大于 31 时只尝试第二保留子槽；占用时仍须保留刚增加的父计数。建立 `146ba` 时 Team 不加 16，属性增量为 `0x70`，仅写 VX/VY 高 word 为 facing0 的 `(+30,-64)`，任意非零 facing 的 `(-30,-64)`，保留这两个 long 的低 word；不写 X/Y、Facing、Pose、Gravity、Owner、TargetMask 等未初始化字节。
- `31` 计数大于 31 时忽略子槽占用，按父 Gravity 最高 byte 0/1/2 调用 `19408/19426/19444`；其他 byte 只保留计数，不构造。后续构造不得重复扣能量，也不得重置计数或 Gravity 低 24 bits。
- 坐标 word、属性 word、速度 neg.l、counter word、FrameIndex/ExtraJump/Layer/Team/TargetBit byte 均须保留原位宽回绕，包含 signed 最小值。
- 不得新增整 RAM、CPU/VM 或重复同址存储。父 `AlternateVelocity` 是 `+48` 唯一 long；父／子 Gravity 是 `+4c` 唯一 long；子 `+fe` 父引用不可借用飞影 DragonBank 的语义。窄 view 的变更须先获 Root 合同确定。
- 所有原 CPU 调用都审计完整 64 KiB RAM。父人物 256 bytes、三个保留子槽 768 bytes、FX 池 2,048 bytes及游标全部精确比较；独立音频消费和 IRQ 需记录实际入口／退出及变更链，不能将无解释的外部字节直接屏蔽。
- 原始 core 的无观察 3,602 视频步控制只能由 Root 产出一次；本 collector 只重放已绑定输入及 control，不擅自另采原版基线。天然调用只计实际来源 5，需保留排空窗口，不得以私有直接调用冒充自然可达。
- 连续流只初始化一次。每步执行原 `19c96` 与独立时钟，之后只从 immutable 动画记录刷新 Duration/Flags；不能回灌 expected Clock、子对象或任何中间状态。每步 movement 参数重新按当前 `+90` immutable source 选择；MAIN 不运行子回调、FX 更新、尾段或世界调度，不把本范围称为完整对局。
- 所有失败场景与 CPU fixture 先于测试；测试冻结后用真实缺少本模块符号的编译红灯证明先行性，随后实现。不得改断言换取绿灯；需保存 native race、Wasm、源码／夹具／日志 SHA 与可复跑命令。
