# 扑克选牌、提示、手机操作与电脑策略修正

本次基线是 v1.30.1（`94588850`），修复斗地主、掼蛋和升级的客户端交互，以及斗地主、掼蛋的提示和用炸策略。规则、计分、地区玩法保持原样。

## 选牌和收牌

选牌身份改为房间、比赛、观察席位和发牌序号，不再使用每次行动都会变化的 `window`。别人出牌、自己不出、轮到自己、忙碌状态和拒绝出牌都不清空选择；手牌变动时仅移除已经不在手里的实体 ID。

进贡／还贡和两款游戏的底牌加入手牌时，新增实体 ID 加入现有选中集合并保持抬起。相同张数的交换也按 ID 差集识别。新发一手、换房、换比赛和换席位时重建选择，首次进入已有手牌不自动全选。

## 提示

- 三带二优先保留完整牌组；主体相同、拆牌代价相同，附带对子按实际点数从小到大，不按花色对应的实体 ID 排序。
- 用户例子 `444 + 55 + 888` 跟三带二时，推荐 `44455`，保留 `888`；完整对 5 和对 9 都可用时优先对 5。
- 持有五张 9 时，完整五张炸弹排在截取四张的建议之前。合法的一手出完优先；手动拆牌和其他合法选项仍可使用。
- 排序特征每个候选只计算一次，避免比较器反复分配统计表。

## 手机界面

倒计时改用两列网格，时间不换行，状态文字允许缩略。验证直接检查时间文字的实际边界，而不是只看外框。

正常出牌的三个按钮使用不可滚动的固定三列。进贡、还贡、扣底和本副结果说明改由说明／结果按钮打开；升级亮主和反主的多个选项放入弹窗，避免挤出底部确认操作。未改变 PC 的整体牌桌框架。

## 公开 AI 参考与适配

参考来源：

1. [DouZero（ICML 2021）](https://github.com/kwai/DouZero)，核查提交 `718a5c920bf3361e34178a38f3b80458e176b351` 的 `douzero/evaluation/deep_agent.py`、`douzero/env/env.py` 及 README。其模型对合法动作估计价值，输入包括自己手牌、公开出牌历史、剩牌数和其他牌的合并信息；目标是长期获胜。现成模型针对普通三人斗地主，不能直接用于上海四人双副牌。
2. [QinlinChen/guandan-ai](https://github.com/QinlinChen/guandan-ai)，核查提交 `c34ae2849c025faadbbc43d7cba6d37bc893120c` 的 `client/ai_client.py`、`client/utils.py`。其规则策略区分是否持有牌权，先对整手牌分组，再考虑队友、控牌和跟牌；不是把“有合法炸弹”直接等同于必须炸。

本次未复制这些项目的实现，也未接入预训练模型或外部服务。各游戏分别实现最多扩展 512 个状态的手牌分组搜索，按炸后的出牌手数、散张、长组合、剩余控制牌、队友接牌机会及公开威胁评估是否值得抢回牌权。上海斗地主还考虑剩余炸弹额度。

无明确目的的中盘跟牌也不再通过拆掉完整炸弹来消耗单张；面对即将出完的对手仍保留紧急拆牌的选择。

控制牌判断只使用自己的牌、公开已出牌和公开剩牌数；其他席位的未知牌合并推导，不读取其实际分布。搜索结果是合法分组的上界估计，不保证所有局面最优。

## 验证与复验

实现前记录失败场景并运行针对性回归；真实 Chrome 房间验证自己不出、其他玩家完成一圈后，选牌仍保留。WebKit 检查三个游戏在手机触屏和 PC 鼠标下的选牌、收牌、拒绝操作、重新发牌、时钟文字边界及操作按钮命中；公开状态投影测试不冒充完整规则对局。

AI 使用同一批 32 副起始牌，交换新旧策略所处阵营，每款 64 局，通过生产规则接口自然完成。冻结的 v1.30.1 策略仅在 `_test.go` 中用于对照；双方使用当前合法候选。最终样本中，斗地主新策略赢 37/64，掼蛋赢 33/64，分别出现 35／36 次炸后仍有五张以上手牌的出炸。这个小样本不证明一般胜率提升，也不把“炸得更多”视作更聪明。用例另外验证中盘两组长牌需要牌权时会用炸，散张多、无明确目的时会让过。

起始牌、完整动作轨迹、结果和耗时保存在 `.local/poker-selection-oct23/ai/<game>/`，可用同一文件重复执行。验收汇总在 `.local/poker-selection-oct23/verification.json`。

```sh
. scripts/env.sh
python3 -B scripts/poker/check-hints.py
python3 -B scripts/accept_poker.py
POKER_BOT_COMPARISON=1 go test ./games/doudizhu/server ./games/guandan/server -run '^TestBotPairedDealComparison$' -count=1 -v
```

WebKit 是 Safari 相关布局的补充验收，测试依赖和浏览器均放在工作区，不进入应用依赖或镜像：

```sh
. scripts/env.sh
PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1 npm install --prefix .local/poker-selection-oct23/webkit playwright@1.64.1 --no-audit --no-fund
PLAYWRIGHT_BROWSERS_PATH="$PWD/.local/poker-selection-oct23/webkit-browsers" node .local/poker-selection-oct23/webkit/node_modules/playwright/cli.js install webkit
python3 -B scripts/poker/check-selection.py --webkit
```

WebKit 的同引擎触摸和尺寸验证不等同于在用户的真实 iPhone 上运行。测试浏览器必须顺序执行，不能并发重建 Vite 使用的公共素材目录。
