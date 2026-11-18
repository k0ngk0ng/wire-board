# 胜负结算及继续路线

胜利保留原版的标题、首领画像、出战队伍和挑战规则。核对了 47 条路线的首领对应关系。普通胜利确认后回到同一路线第 1 波，恢复 14 条生命并保留部署。挑战胜利在确认前完成奖带和出战者统计；确认时卸装、还原形态并收回队伍。七条秘密路线在确认胜利时各授予一次伙伴。

战败窗口允许从第 1 波重开。第 25 波起也可从上一个检查点重试；第 100 波重试仍是第 100 波且只有 1 条生命。部署保留，旧波次的移动锁、压力和计时清零。永久死亡重开会结束挑战。自定义练习沿用原版的成功或失败提示和画像，重开仍留在练习路线。

结果冻结在共享规则状态中。确认前拒绝开波、换图和其他场上操作；观战只能查看。确认后更新控制代次，旧请求无法重复领奖。混合房留给真人决定，全部席位托管时可用相同的规则入口继续路线。

```sh
. scripts/env.sh
npm --prefix web run build
node games/pokepath/tests/result-render-oracle.mjs
node games/pokepath/tests/native-platform-browser.mjs --standard-web --result-victory
node games/pokepath/tests/native-platform-browser.mjs --standard-web --result-defeat
go test ./games/pokepath/tests -run '^TestNative(ProgressionRouteResult|ProgressionChallengeResult|ProgressionDefeatRetry|ProgressionSecretRouteVictory|PlatformResult)' -count=1
```

对照夹具采用 `FinalScene.js` 与 `DefeatScene.js` 的原版渲染片段，并按 `GameScene.open` 的层级挂到游戏画面。所有 17 组冻结画面及操作回执一致；截图和未遮罩报告位于 `.local/pokepath/verification/result-render/result.json`。正式网页的两席位与观战测试分别验证桌面确认胜利、320 px 手机触控重试、原塔保留和开出下一波；报告在 `.local/pokepath/native-browser/run-8e18e911/` 与 `.local/pokepath/native-browser/run-7df41171/`。这些对局用测试专用快照准备最后一波入口，并走真实的引擎结算、平台鉴权和操作链路。电脑百波另从正式初始状态运行验证。
