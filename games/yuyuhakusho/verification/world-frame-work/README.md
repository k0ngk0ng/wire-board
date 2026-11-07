# 世界帧前置的工作边界与共享字段

逻辑帧的计数、慢动作、暂停、随机数和反馈现在由同一份语义实现推进。旧纯值 API 委托这些公共实现；新的 `BeginWorldFrameWork` 可接入实际归一化与同步工作 hook，`AdvanceWorldFrameOnOwners` 则借用已经分叉的实际字段，供联合主时钟使用。

每个操作仍使用现有 `MainWorkOperation`，包含实际分支、循环、位移、16-bit wrap、DIVU 数据相关成本及内部 refresh 顺序。代码不执行原程序或保存操作历史。归一化是必须显式提供的调用，慢动作分支跳过时才不消费输入；未提供归一化不能按零成本继续。

两个固定核心各完成 4,568 个真实调用、71,555 个工作边界。原先冻结的 4,536 组计数／暂停／反馈结果逐项保持不变，另有 32 个 RNG 种子与入口时钟组合。新实现逐点比较时刻与 refresh，并比较完整结果；每个实际操作注入失败后保持原输入，后续 hook 停止。原生预期及来源步骤保存在本目录，旧 fixture 未修改。

`ChuWorldHooks.Owners` 在内部值／克隆边界借出当前事务的 World。前置可直接更新同一个 Clock、Controllers、RNG；中途声音确认的 AudioPause 和 VINT 的 Palette.Flags 更新得以保留。`session.WorldFrameWithHooks` 对应提供 Prefix、Normalize、Owners，音频与世界一起分叉，晚期对象或调色板失败时一起撤销。这些借用与 hook 均不进入存档。

实现前的 [FAILURES.md](FAILURES.md)、[test-first.json](test-first.json) 与 [live-test-first.json](live-test-first.json) 保留原测试及输入。首次测试中，暂停转换探针原本希望在 producer 写入前注入确认，却在每个操作及最后 return 都改成 FF；修正为只在前置操作注入。期望值、原版数据和生产实现均未改动，原测试及准确差异见 [pause-hook-adapter.json](pause-hook-adapter.json)。独立 hold 探针仍在每个操作写入确认，证明它不会被旧副本覆盖。

整段真实 3A2E 前置与主时钟的联合验收、最终命令和日志见 [world-logic-begin](../world-logic-begin/README.md)。本目录的 helper 入口是明确给定的条件，不能单独代表整个世界或比赛的独立时序。

后续给角色／碰撞增加任意工作介入点时，还须处理各组件的 SoundQueue、Palette.Flags 等旧视图回写，以及 IRQ 可读滚动字段的即时发布。当前共享前置与 owner 绑定不表示这些对象成本已经完成。
