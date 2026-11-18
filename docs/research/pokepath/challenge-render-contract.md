# 挑战菜单与合作轮抽对照

挑战菜单读取当前路线、已获奖带和路线奖励。七条规则按原版顺序循环。前三条各有两档，其余四条为开关。当前说明随鼠标经过、键盘聚焦或触控切换；等级限制和禁用道具显示难度说明，轮抽显示首领削弱说明。

每个客户端保留自己的未提交选择。开始挑战后，所有玩家都看到服务器确认的设置，并按同一个候选池轮抽。候选操作带当前轮抽代次；上一轮的选择不能落到新候选上。挑战期间禁止切换路线；轮抽挑战锁定仓库，禁用道具规则锁定背包。鼠标、键盘和手柄经过同一个面板限制。

奖励按 1、2、3、5 枚奖带的门槛排列，提交编号分别为 `3、0、1、2`。已经持有的奖励伙伴按收集编号识别。丢失的可装备奖励和不再持有的奖励伙伴可重新领取；已领过的金币奖励不能重复领取。奖励门槛、持有和领取均由 Go 状态裁决。观众可查看规则和奖励。

```sh
. scripts/env.sh
npm --prefix web run build
node games/pokepath/tests/challenge-render-oracle.mjs
node games/pokepath/tests/native-platform-browser.mjs --standard-web --challenge
go test ./games/pokepath/tests -run '^TestNativeProgression(ChallengeDraftAndReward|NonDraftChallenge|Permadeath)' -count=1
```

画面报告在 `.local/pokepath/verification/challenge-render/result.json`。参考端仅加载原版 `ChallengeScene` 和奖励区的 DOM 方法，正式游戏使用独立组件。16 个固定画面的行为和操作回执一致。规则行、说明及奖励位的测量一致；奖带像素还有差异，整组严格像素对照未通过。

四席位浏览器验收使用专用测试快照准备解锁条件，之后通过标准平台的操作入口开始挑战、领取奖励及完成六轮选择。四个独立玩家账号共享结果，观众无权修改，过期轮抽输入被拒绝。第三位玩家以 320 px 视口触控选择。报告在 `.local/pokepath/native-browser/run-*/result.json`。

轮抽窗口和波次运行中开启新挑战的对照见 [轮抽界面、候选与挑战重开](draft-render-contract.md)。底栏显示、暂停／续玩和波中退出见 [挑战计时与波中放弃](challenge-clock-contract.md)。
