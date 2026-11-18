# 挑战计时与波中放弃

原版在挑战开启时将计时器归零，第一波开打后才开始累计，以 `时:分:秒.百分之一秒` 显示。间歇也计时；快进只改变战斗速度。手动暂停、战败和胜利画面停表。普通重试保留原累计时间，永久死亡重试结束挑战。

平台把首次开波的逻辑时刻存入同一份权威快照。所有席位从相同起点与规则时钟计算显示，保存确认期间不累计，离线后继续也不会补计。边界不由各端的系统时钟、页面可见性或弹窗打开时间决定。非挑战底栏继续显示普通波次收入；自定义练习与挑战沿用原版的隐藏条件。

放弃挑战只移除部署及其专属弹道，收回伙伴，解除装备和超级形态。已经出生的敌人保留当时的路线、位置、生命和本波属性；正处于波中时继续推进原有战斗。未结算的本波金币、生命状态和自动下一波开关保留，后续开波不再应用挑战规则。规则转换更新控制代次，暂停或战败时不能用放弃跳过该状态。

参考源为 PokéPath TD v1.7.3 的 `Utility.js`、`ChallengeScene.js`、`Area.js` 和 `ui.css`；研究实现只进入对照夹具。六个底栏场景全部逐像素一致，报告在 `.local/pokepath/verification/challenge-clock/result.json`。

正式网页的四个席位及观战账号在暂停时都显示 `00:00:01.30`。四位玩家随后看到放弃挑战后的原敌人继续推进，手机底栏无重叠。浏览器报告及截图位于 `.local/pokepath/native-browser/run-c679e4ae/`。服务端快照测试另外验证了保存确认停表、模拟离线 48 小时后不补计，以及恢复后的精确推进。

```sh
. scripts/env.sh
node games/pokepath/build/rules-revision.mjs --check
go test ./games/pokepath/tests -run '^TestNative(ProgressionChallengeTimer|PlatformChallengeTimer|ProgressionSurrender|ProgressionChallengeVictory)' -count=1
npm --prefix web run build
node games/pokepath/tests/challenge-clock-oracle.mjs
node games/pokepath/tests/native-platform-browser.mjs --standard-web --challenge
```

计时冻结和奖带发放已通过规则测试。胜利／战败窗口与关闭后的续关流程见 [胜负结算及继续路线](result-render-contract.md)。
