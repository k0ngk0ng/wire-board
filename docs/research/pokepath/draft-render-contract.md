# 轮抽界面、候选与挑战重开

轮抽弹窗使用原版标题、轮次和三列布局。候选以持有记录编号标识，显示该记录的别名及实际精灵图。未补满三位时保留空位。每位候选有独立的详情按钮；其他玩家换掉这一轮候选后，旧详情会关闭。观众可以看详情，不能选择、重抽或重启。移动端扩大操作区域，保留三列顺序。

服务端在洗牌后保存候选池顺序。开启挑战会先把出战伙伴依次放到现有仓库末尾，再洗牌。首轮保底单独占一列，其余地图保底在剩余池中寻找；候选不会重复。重抽更新轮次代次但不增加轮次。如果重抽结果含原版三个返还机会的收集编号之一，可以再重抽。重新开始把已选伙伴送回共享候选池。选择、重抽和重启都要求当前代次。轮抽开始后，战场、商店和伙伴修改暂时不可用；候选详情保留关闭按钮。

已解锁的路线可在未暂停的战斗中开启挑战。服务器回到第一波，清空旧敌人、塔位、弹道及本波统计，保留收藏、原有金币和长期统计。战场控制代次更新，旧波次产生的输入不能落到新挑战上。

```sh
. scripts/env.sh
node games/pokepath/build/rules-revision.mjs --check
go test ./games/pokepath/tests -run '^TestNativeProgression(DraftOriginalPrecedenceAndSharedRestart|ChallengeReplacesRunningWave)$' -count=1
npm --prefix web run build
node games/pokepath/tests/draft-render-oracle.mjs
node games/pokepath/tests/native-platform-browser.mjs --standard-web --challenge
```

原版 `DraftScene.js` 的 SHA-256 为 `fd5e262b083b2e636b1f614b3384f10e4080b13ebba2207d8d2eebc0b965cd08`。浏览器对照只将其渲染和受控候选方法用于隔离夹具；产品使用 React 组件、Go 权威规则及同源 Wasm 预测。

按原版 `GameScene.open` 的挂载层级重新对照后，15 组画面的内容、操作回执及像素全部一致。对照结果在 `.local/pokepath/verification/draft-render/result.json`，保留完整截图和未遮罩的像素差。四席位结果验证了波次运行中开启挑战、跨席位重抽与重启、关闭旧候选详情、观战只读，以及六轮选择，其中一轮用 320 px 触控完成。报告在 `.local/pokepath/native-browser/run-*/result.json`。
