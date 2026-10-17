# 原版对手交互 AI 验收

实现前失败场景：

- 模式 7 进入时没有清零跳跃阶段并设计数 8，后续帧误清零。
- 获得球权后未切持球等待；搭档连接字节的高位与目标身份检查错误。
- 低四位分派与完整参数动作查表混淆，越界动作参数被静默取模。
- 站立攻击分支遗漏停止平面速度、面对已选目标、强制重新开始动作；错误直接结算命中或伤害。
- 普通跳跃分支混淆首次请求与已跳跃状态；空中上升速度使用有符号比较，提前触发攻击。
- 落地后攻击分支混淆落地动作与落地已进入标志，直接改当前动作而非下一动作。
- 跑跳攻击分支未保留高度低字节 16 阈值、首次选择跳跃时误立即攻击。
- 不同分支错误消耗随机数、替换已选择目标或改变计划；未支持参数不明确报错。

证据：bank 5 `$A2E9 → $AE96` 的 12 项分派表和 `$AF90` 动作表；REA 批量输出保存在 `.local/basketball/rea/engagement/`。全部原始模式 7 样本纳入，不筛掉不匹配分支。攻击仅产出动作／停止命令，物理与命中留给世界步骤。

## 本次结果

- REA 初始批量证据 `ev_a9d2e1e30ec7997f999aeab45fa2e8ad0d22e6d901add4e5409fc3c794200402`：9 个函数查询中 7 个成功；`$AEE8/$AF4E` 返回加载映射错误。针对两处改用独立种子重新分析仍失败，保留证据 `ev_10c214df8b812059218e84e12de74c33006c4f0ac2e163fc8e0c59aaf65411e4`，不声明两处已成功反编译。
- 对两处失败及 carry／间接分派语义，使用同一目标字节核对指令。`source-instructions.txt` 保留 SHA-256、跳转表、动作表和指令，和自然执行结果共同约束实现。
- 全部 206 条原版模式 7 调用逐字段匹配，参数覆盖 0、1、3、4、6、7、10、11；含 4 次下一动作实际改写及 6 次平面速度实际清零。没有筛掉任何模式 7 调用。
- 预先创建的固定规则测试覆盖跳跃首次选择、速度 2／3／255、落地未进入／已进入、跑跳高度 15／16、球权／搭档连接交接以及未知参数拒绝。固定案例不是自然样本。
- 测试先于实现，红灯与通过日志为 `test-before.log/test-after.log`，竞态检查通过并保留 `race.json`。`AIEngagementCommand` 只传递原 AI 的停止平面速度和下一动作命令；没有实现或伪造攻击碰撞、受伤、得分。

## Court0非人物目标：实现前失败清单

- 固定全电脑冷局在video4952/logic1648，actor2 mode7 parameter0 trackedObject=7（篮筐7）；不能把该真实目标强改为人物。
- 原B1E9按目标索引读取16位整数X；小数必须忽略，等于时FacingLeft，不能使用屏幕坐标、 signed X 或近似距离。
- face只负责Heading/Facing；保持TrackedObject、随机数与计划；调用分支原有停止/攻击命令保持顺序。
- 已恢复的canonical别名源可复用原坐标读函数；尚无来源的目标保持明确错误，不从零值模拟RAM。

结果与复跑：

- REA `ev_acea39eea50c9f4d0e8d88d628d86e308a4ff819486b40cbac3d329a1df4eaca`，A2E9/AEB6/B1E9成功解析，输出 `.local/basketball/rea/prop-kind10/engagement-analysis/`。
- `engagement-hoop-isolated.json`包含48条未修改原A2E9机器码执行，四个actor均以原target7完成停止、面向和动作请求。覆盖等整数不同小数、目标两侧及16位整数高位。TrackedObject没有改写。
- 新代码只把face的「仅人物」限制替换为已审计的`originalPursuitX`坐标来源读取；原12分支、随机数、计划与控制目标保持。其余来源仍按既有错误处理。
- 原206条自然mode7调用、新48条隔离调用和原边界测试通过；接触/profile/engagement共26项定向race通过，日志 `.local/basketball/rea/prop-kind10/race-contact-engagement.json`。
- 固定Court0全电脑video4952/logic1648前态保存在 `.local/basketball/rea/prop-kind10/all-computer-before-engagement.json`。同期显示列历史的empty/nil恢复问题另由其维护者修复后，两次完整冷局均通过：22407video、7443logic、84次恢复、比分52:29，轨迹SHA均为`26eca49b43278bbfa82b2534d573cfb2c6f7779d3ff5ad026e60924296265f98`。最终报告归档在 `.local/basketball/rea/prop-kind10/court-zero-final-runtime.log`，注明原报告来源和SHA；这仍不等于原ROM整局逐帧一致性证明。

```sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_engagement_hoop_original.py
source scripts/env.sh
go test -race ./games/basketball/engine -run 'TestOriginalEngagement' -count=1 -json
go test ./games/basketball/engine -run '^TestRuntimeCourtZeroAllComputerCompleteMatchAndRestore$' -count=1 -v
```
