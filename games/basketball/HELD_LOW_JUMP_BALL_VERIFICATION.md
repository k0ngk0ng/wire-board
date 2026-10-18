# 低跳投篮中的持球动画

原 bank6 `B598..B5B7` 会按持球者的动作选择球动画游标：动作 `5A` 使用人物游标＋2，动作 `5C` 使用人物游标＋9。此前普通持球分支只显示一个固定球姿态，遗漏了原镜像、隐藏和旋转阶段。

先行测试覆盖原完整 `B585` 调用：四名人物、两种低跳动作的全部合法游标、首入／已进入、计时器绕回、姿态边界、左右镜像、人物换动作后的球动画延续，以及球持有自身时遗留序列的继续。修改前首差为球的镜像标志，失败日志已保留。

实现复用原语义动画结构，恢复正常帧、两段低跳序列及其回到全序列起点的循环；不会把 `elapsed` 的 uint8 绕回误认作序列结束。没有改球权、重力或自动结算策略。3,210 条原调用及已有持球／自身持球／完整世界回归在 Native、race、Wasm 均通过。

```sh
source scripts/env.sh
python3 -B scripts/basketball/verify_held_low_jump_ball.py --write-fixture
go test ./games/basketball/engine -run '^(TestOriginalHeldBallLowJumpCalls|TestSelfHeldBall|TestSelfDribble|TestCatchHeldBall|TestWorldActionsEntireCapturedCorpus)' -count=1
```

结果索引：`.local/basketball/rea/held-low-jump/result.json`。实际平台原模式 128 的浏览器完整比赛另通过 34 项检查，正常电脑选择计划 12，记录了 16 个真正进入可见绘制列表、位于视区内的低跳姿态及 Canvas 截图。原位图仍由已有原素材渲染，不将模拟状态刚变化但尚未提交显示的帧冒充可见动画。

浏览器入口：`python3 -B scripts/basketball/accept_live.py --court 1 --full-match --all-computer --observe-plan12 --match-mode 128`，产物位于 `.local/basketball/all-computer-full-match-browser-plan12-mode128/`。这是完整单场与所观察动作的验收，不代表完整战役或所有特殊动作画面都已通过。
