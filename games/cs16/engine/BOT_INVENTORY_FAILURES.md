# 电脑装备输入失败场景

本清单与 E2E 先于策略实现。自然 `fy_iceworld` 检查点已经出现两项失败：部署冷却时的第一次 Reload 被规则拒绝，但电脑一直保持 Reload 为 true，无法产生新的按下边沿；主枪弹匣与备弹都为空时仍持续开火，忽略库存中有弹药的手枪。

- 真实 3 帧采样调用 BotInput、QueueInput 和规则推进时，冷却中被拒的第一次装填须先释放；达到 NextAttack 且当前没有装填、已提交 Reload 为 false 后，再产生新的有效按下边沿。
- active Reload 和尚未到期的部署／攻击冷却期间输出应释放 Reload，不靠修改 ReloadUntil、NextAttack 或弹药跳过规则。
- 装填完成后须出现真实 reload_end 与实际 shot，不得只证明 ACK、输入布尔或库存数值变化。
- 每次新决策和 cached 输入都要复查当前真实库存，不能重放被拒的 held Reload 或给已经耗尽的主枪继续开火。
- 可用主枪优先，其次有弹匣或备弹的副枪，再其次小刀；换枪只通过 Select，不直接写入 Weapon、Inventory 或 Ammo。
- 切换到新武器的首个输入不得同时重放旧武器的开火／模式／装填；之后沿原电脑的距离、半自动节奏和友军射线检查执行。
- C4 下包及已有的投掷物／小刀明确选择不能被装备修正覆盖。
- helper 只修改将要提交的 Input；权威 State、随机种子、已有输入和库存保持不变。

本轮只修合法装填脉冲与副枪／刀切换，不增加丢空枪重捡等复杂策略。所有产物位于 `.local/cs16/verification/bot-inventory-*.jsonl`；实际原图自然对局由父任务统一复跑。
