# 原版角色攻击 handler 25

实现前先列出并采样失败场景：误用球位置代替当前持球者位置、覆盖演员自身小数坐标、负高度整值钳零而不是仅清高字节、持球者就是演员本人、动画重复进入与提前传送。先保留失败测试，再修改实现。

原版 `bank6:$A471` 在攻击动画结束、动作变为 Jump 时，复制当前球持有者的整数 X、Depth、Height，保留演员自身小数部分。Height 最高字节为负时只清该字节，再加 8 像素；清纵向速度和重力后设置重力为 1，JumpPhase 为 FF。普通世界攻击从当次世界状态的真实 Holder 绑定位置，不从显示快照推测。

- REA 分析沿用 `.local/basketball/rea/player-attack/analysis/normalized.json` 中 `$A471`，ROM 摘要保留在测试来源 JSON。
- `verify_attack_handlers.py --handler 25` 产生 608 次未修改原指令调用，其中 224 次覆盖全部 7 个持有者编号和负高度、进位、小数边界。连续调用保留原动作自身生成的状态。
- `TestOriginalAttackHandler25` 与旧攻击测试的 Native、race、Wasm 验证通过；日志与摘要：`.local/basketball/attack-handler-25/verification.json`。
- 这只是隔离原函数与生产攻击调用的验收，不是整场游戏覆盖。球队 7 实际固定种子比赛越过原 handler 25 缺口后，在 video 2261、logic 751 遇到尚未恢复的 handler 10；失败状态保留在 `.local/basketball/team7-after-handler25-20261018/native/`。

复现需先 `source scripts/env.sh`，再执行 `go test ./games/basketball/engine -run '^TestOriginalAttackHandler25$' -count=1 -v`。原指令 fixture 生成需使用仓库已有 Python 依赖路径，不下载依赖、不改变原种子或时钟。

## 后续普通攻击恢复

- handler 10（原 A83C）：1,568 次原调用匹配。每次调用都发原攻击声音，保留纵向运动，动画后先检查落地再推进物理。
- handler 17（原 A945）：672 次原调用匹配。保留旧显示朝向确定水平速度，三次落地弹起按原循环计数处理，每次实际落地发声音 23；不添加额外入场声音。
- handler 20（原 A9C8）：416 次原调用匹配。下降时从循环动画跳到索引 5 的下劈尾帧，使用当前显示姿态决定 8 像素水平速度，纵向速度设为 -8、重力归零；物理后判断落地。

## 命中后的动画尾段

原 combat 反馈会将攻击 cursor 加 2，跳过常规结束符后进入另一段动画。原导出器只保留首个结束符之前的帧，导致合法命中后报 cursor 越界。现为 variant 57／87／94／98 和下降攻击 variant 15 显式导出尾段；保留原绝对 cursor，尾段结束仍回原索引 0，不修改公共动画范围。

- 504 次原 contact → attack 连续调用与 13 次尾段碰撞形状读取匹配。variant 98 尾段原形状同时禁用攻击和受击框，其他尾段分别使用原 $9139、$9149；不以空形状兜底。
- 碰撞允许范围受原球队、动作、variant 选择表约束，不放开任意 cursor。原 action 68／variant 63 释放尾段保留其单独范围。
- 初版研究脚本沿用 `initial` 的 raw heading 1，解码为语义方向时会损失该非标准字节；保存失败日志后，将这组链的起始方向显式设为原右 64／左 192，下一动作设为该 handler 原初始化的 Idle。没有修改原指令或实际完整赛失败存档。
- 球队 3 两局固定种子比赛通过：每局 22,397 video、7,437 logic、84 次存档恢复，轨迹摘要一致，sourceStable=true；记录 `.local/basketball/team3-after-contact-tails/native/report.json`。这不代表全球队、全部角色或全助攻玩法验收完成。

## 普通助攻起手与队友发射

普通 action 09／0B 原分派为 9852／8E5E；576 次原调用和连续链匹配。09 完成后进入 0B，0B 将关联队友切换为 3E 并清关联，后续发射逻辑仍由原 3E 处理。X 摩擦截零，Depth 摩擦允许跨零；09 的备用重力在运动后才加上。

0569 等于本演员时的 carried-body 09 双人联动物理（9C4E）随后独立恢复：先复制本演员速度到支撑物，推进支撑物，再推进附着演员，复制支撑物经运动修正的速度回演员，最后清支撑物速度与重力。后续附着 0B 也必须走原 EEFF；Court1 的普通演员调度使用 FlatCourt，需要显式绑定支撑物。先在测试使用真实调度路径复现错误，再修复绑定。

1,440 次原完整人物调用匹配，覆盖 09 配对、0B 附着释放、0B 关联 FF 原写入 $0611（演员 0 Endurance=3E），支撑物 0..5、场地 0/1/2、首次／持续动作和两种朝向。576 次普通助攻与 1,440 次配对调用是独立来源，不能计作 2,016 场比赛。

初次研究输入从纽约 RAM 继承道具 kinds，却把 Court 改为 0/2，造成与语义世界固定场地 kinds 不一致；失败日志已保留。随后 fixture 的初始化同时选择原场地对应 kinds，再执行不变原指令。此修正不影响任何真实比赛失败状态。

**仍未证明全部助攻玩法完成。** 这些是隔离调用与普通连续链验收，后续 3E 完整发射、所有场地和双人自然操作链需单独验收。生产纯函数 `StepPlayerAssist` 仍明确拒绝单独执行 carried-body 分支：该分支由世界适配器 `advanceWorldPlayerAssist` 按原顺序执行两对象运动，不可只推进一个玩家。

最终本域回归命令与 Native race／Wasm 结果保存于 `.local/basketball/attack-handler-25/handoff-*.log`，并覆盖旧原攻击、combat、普通碰撞几何和 action 68 释放尾段。其通过不代表其他并行开发模块、浏览器构建或全部球队已完成。

球队 6 后续使用固定原种子复核，两局均通过：22,373 video、7,429 logic、84 次存档恢复，sourceStable=true，轨迹一致。记录 `.local/basketball/team6-attack-handoff-20261018/native/report.json`；先前 sourceStable=false 的运行仍保留，不替换为通过。最终本域汇总：`.local/basketball/attack-handler-25/handoff-summary.json`。
