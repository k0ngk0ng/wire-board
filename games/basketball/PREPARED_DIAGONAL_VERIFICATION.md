# 准备配置暴露的斜向发射与纽约球体运动

完整准备配置矩阵中的 6 个失败前态保存在 `engine/testdata/preparation-diagonal-runtime.json.gz`。先增加 `TestPreparedDiagonalRuntimeFirstDifferences`，确认 4 个前态停在斜向发射保护、2 个停在纽约高台接触保护；失败记录为 `.local/basketball/rea/preparation-matrix/diagonal-before.log`。这些是固定种子 `0x12345678`、正常时钟运行产生的原生复刻状态，不能称为原版自然对局。

原 bank 5 `$8C23` 发射函数调用 `$9728` 计算方向。正 X、负纵深且两个距离相等时，`$97C9` 的实际字节为 `A5 69`，即读取 `$0069`；其他等距象限使用 `A9 69` 读取常量。`$0069` 是人物席位 1 的 X 小数位。因此这一象限的距离系数依赖另一人物的当前位置，不能统一替换成 105。`PassLaunchContext` 显式保留该来源；世界调度中的闭包在实际发射时读取席位 1，保留之前已经发生的人物更新。缺少此上下文的独立调用仍明确拒绝该象限。

`verify_diagonal_launch_original.py` 使用未修改的原 `$8C23` 指令执行 576 组隔离输入，覆盖四象限、零距离、距离 1／13／64、6 个不同 X 小数值、3 个源高度和 2 个弧度。输出速度及重力与 Go 实现逐项比较；ROM 和夹具 SHA-256 见 `engine/testdata/diagonal-launch-function-source.json`。测试先出现缺少上下文接口的失败，再补实现；记录为 `launch-before.log`。

纽约世界调度仅将球对象 6 接入已恢复的完整 `$EEFF` 运动，包括原地形查询、边界、下落和最终位置提交。人物、道具与篮筐的纽约路径保持原有范围。没有删除旧近似路径的保护后继续运行。此前 `$EEFF` 的 REA 证据见 `.local/basketball/rea/pass-floor/analysis/`，相关恢复及 13,229 次自然原函数调用对照见 [COURT_TERRAIN_VERIFICATION.md](COURT_TERRAIN_VERIFICATION.md)。

`verify_prepared_corner_motion.py` 将两个失败前态的球体数据放入已有原纽约 RAM 上下文，执行未修改的 `$EEFF`。这是明确注入输入的原函数隔离验证。两例均按原指令落到高度 0，并保留水平运动；检查完整原运动状态及绑定后的球体输出，不能只检查不再报错。输入、输出与摘要保存在 `engine/testdata/prepared-corner-motion*`。

```sh
source scripts/env.sh
python3 -B scripts/basketball/verify_diagonal_launch_original.py
python3 -B scripts/basketball/verify_prepared_corner_motion.py
go test -race ./games/basketball/engine -run '^(TestPreparedDiagonalRuntimeFirstDifferences|TestDiagonalLaunchAgainstOriginalFunction|TestWorldActionsEntireCapturedCorpus|TestCompleteOriginalCourtMotionNatural|TestOriginalCourtFloorNaturalNewYork|TestPassLaunchAgainstOriginal|TestShotLaunchAgainstOriginal|TestShotFlightAgainstOriginal|TestPassFloorAgainstOriginalFunction|TestOriginalSpecialBallAndHighTargetTrace|TestWorldTeamSpecial.*|TestHoopActionsAgainstOriginal)$' -count=1 -v
```

验收日志位于 `.local/basketball/rea/preparation-matrix/diagonal-original-regression.log`、`diagonal-race.log` 和 `diagonal-wasm.log`。覆盖 6 个真实 `StepVideo` 首差及序列化后重放、576 次原发射、2 次原角落运动、14,458 次自然地形、13,229 次完整自然运动，以及 1,998 个完整原动作阶段。首差恢复和原函数回归不能替代准备配置的完整比赛验收；完整比赛矩阵由独立运行记录确认。
