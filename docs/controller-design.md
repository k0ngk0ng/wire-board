# 游戏动作与实体手柄设计

## 已确认的方向

- 每款游戏独立保存个人操作方案，并提供默认预设；不能改一个游戏便覆盖另一游戏。
- FC／MD 等复刻游戏保留逻辑按钮布局，普通游戏使用实际动作名称。
- 棋牌与地图类支持方向导航、选择和确认。方向只移动焦点，执行操作仍由原游戏界面及规则确认。
- 平台统一处理键盘、手柄按钮、摇杆、冲突、连发和生命周期；手柄型号识别与游戏动作含义分离。
- 房间操作中全屏在最左侧，离开观战／离开房间在最右侧。头像与用户名合并为一个玩家页面入口。
- 控制器规范统一维护在 `rules/controller.md`。

## 分层

1. 游戏元数据声明动作 ID、名称、输入类型和默认预设。复刻按钮只是预设类型，不是所有游戏的公共动作词表。
2. 个人绑定按用户和游戏保存；实体手柄映射继续按浏览器报告的型号区分。旧的布局配置保留，通过明确迁移初始化相应游戏，不持续共享可变配置。
3. 公共输入层输出逻辑动作和连续轴值。按下／松开、持续蓄力、连发、模拟转向等必须保留各自语义，不把所有摇杆都压成键盘事件。
4. 游戏适配层把动作送进现有操作与预测接口；平台不改游戏规则或直接改得分、位置和结果。

## 棋牌与地图导航

- 手牌：左右切换相邻牌，上下切换行；确认只切换选中状态。出牌通过单独的出牌操作完成。
- 棋盘／地图：方向切换相邻目标，确保焦点进入可视区域；确认选择地块或棋子，再使用游戏原有的落子、移动、建造等操作。
- 公共导航优先使用已有可聚焦控件；复杂地图提供定位／平移适配，不能只把焦点移到视区外。
- 焦点样式不改变布局、不叠加身份编号；快照更新保留焦点与选择，目标消失时才恢复到合理位置。
- 游戏内弹窗与导航有独立上下文，控制器设置和文本编辑不触发游戏动作。不能通过伪造键盘事件绕过这些边界。

## 接入与验收顺序

1. 实体手柄基础：标准／自定义映射、配置隔离、重新接入、死区、连发、混合输入和真实房间链路。
2. 按游戏的动作元数据与个人方案：迁移现有控制器消费者，提供普通游戏的默认预设与改键。
3. 动作类适配：接入现有键盘动作、连续轴和按住／松开接口。
4. 导航类适配：覆盖手牌、格子、地块、操作区及相应确认流程。
5. 按影响范围验收并保存可重复运行的结果；虚拟硬件来源与实体设备实测分开记录。

本文记录设计与接入计划，不作为上述项目均已完成的证明。完成状态以代码、测试结果及浏览器产物为准。

## 当前接入与验收边界

控制器设置已按用户和游戏保存；实体映射再按浏览器报告的型号隔离。旧的 FC 布局偏好只用于首次迁移，之后各游戏独立保存。未接入的游戏暂不展示操作方案。

- 热血篮球、魂斗罗、乒乓球：接入现有输入循环，合并键盘、触屏和实体手柄来源，保留原有预测与服务端接口。乒乓球使用挥拍等实际动作名称。
- 泡泡堂、坦克大战：使用公共 `useGameController` 读取当前游戏配置，在原有输入与预测循环中采样。选角／选坦克阶段切换到方向导航；比赛中分别保留单次放水泡和持续开火的语义。改键后不再调用旧的固定键处理器，触屏保持独立来源。
- 俄罗斯方块、街霸方块：接入移动、旋转、软降与硬降，俄罗斯方块另有暂存。保留各自重复间隔，旋转／硬降／暂存按下只触发一次；街霸方块选角复用导航。街霸方块仅在前端预测当前落块及投影，按该游戏的碰撞与旋转规则呈现，不在本地落定、消除、计分或生成下一块。
- 疯狂坦克：接入选坦克、移动、炮角、朝向、换弹、四个道具槽与镜头操作。主动松开发射，输入中断取消蓄力；保留鼠标瞄准、拖图、滚轮和触屏蓄力。移动按下时立即提交第一步，持续按住沿用原节奏，短触不会等待轮询后丢失。
- 国际象棋、中国象棋、围棋、连连看：接入棋盘焦点、选择、取消与原有确认流程；和棋复选框可用手柄操作。围棋保留计分／标死子阶段，连连看移除图块后就近恢复焦点。
- 掼蛋、升级：接入手牌多选、分组、出牌、游戏内下拉选项与独立改键；升级验收包含亮主和扣底。
- 极品飞车：接入模拟转向、油门、刹车、手刹、后视、地图、复位和外观切换。连续值按游戏元数据声明，采样沿用渲染循环；可选 `brakeAmount` 向后兼容原布尔刹车，并由服务端与本地预测共同使用。
- 强手棋、军棋、三国杀：接入地图／棋子／卡片与操作区导航。强手棋验证竞价及交易的金额、资产和提交；军棋验证两人、四国布阵与走子；三国杀通过独立测试账号完成选将、出牌与杀的响应。数字字段调整只更新原有草稿，键盘编辑保持独立。
- 五子棋、斗地主、璀璨宝石：首批接入公共方向导航，确认调用既有选择与确认流程。五子棋选点与落子分开；斗地主支持多选、取消与上下换行；璀璨宝石选择卡牌后打开原有详情操作区。
- 卡卡颂、卡坦岛、铁路环游：接入地图与行动分组，选地／选牌和最终提交分开。卡卡颂适配 SVG 镜头跟随，鼠标拖动后仍可用手柄确认；卡坦岛覆盖基础建设与探索者确认面板；铁路环游覆盖选任务、路线详情与铺设支付。
- 征服时代、星垒：接入领地／建造点导航和镜头跟随。征服时代验证征兵、进攻计划、撤销、筑堡及提交，并为直接建城镇提供独立可改键动作；星垒验证建造、升级、优先攻击、卖塔、重建和暂停操作。
- 斯诺克：接入粗细瞄准、模拟摇杆、按住调整力度与松开击球、D 区摆球、击点、菜单和判罚选择。断连与设置切换取消蓄力，保留仍按住的其他输入来源；手机摆球方向跟随竖屏球桌。原生键鼠、触屏和虚拟手柄已在真实房间验收。

验证入口：

```sh
node --experimental-strip-types --test web/tests/controller-input.test.mjs web/tests/gamepad-input.test.mjs
python3 -B scripts/controllers/gamepad/check-browser.py --out .local/gamepad/combined-acceptance --games basketball,tabletennis,gomoku,doudizhu,splendor
python3 -B scripts/controllers/gamepad/check-browser.py --out .local/gamepad/contra-final --contra
python3 -B scripts/controllers/gamepad/check-browser.py --out .local/gamepad/native-touch-final --games battlecity,bubbleparty
python3 -B scripts/controllers/gamepad/check-browser.py --out .local/gamepad/blocks-touch-second --games tetris,spf2t
python3 -B scripts/controllers/gamepad/check-browser.py --out .local/gamepad/fortress-acceptance --games fortressblue
```

浏览器验证需先使用 `scripts/controllers/gamepad/vite.config.mjs` 构建私有前端；魂斗罗另需运行 `scripts/contra/browser_build.py --out .local/gamepad/contra-build`，确保程序资源与当前源码一致。每次执行使用新的 `--out` 目录。HTTP 配置与迁移测试位于 `internal/server/controller*_e2e_test.go`；所有临时目录、Go 缓存和浏览器 profile 均放在工作区内。

浏览器产物包含 `report.json`、截图和真实房间快照，硬件输入使用明确标注的虚拟 Gamepad API；不构成具体实体手柄型号的实测认证。

最近的公共输入回归记录：`.local/gamepad/charge-final-regression/report.json`，源码与检查范围见 `.local/gamepad/charge-acceptance-manifest.json`。此次同时修复俄罗斯方块收到确认后的单帧重复预测：在呈现前按快照确认序号过滤已确认批次，逐帧验收不得出现额外移动。

棋牌导航的最新回归见 `.local/gamepad/navigation-complete/report.json`，覆盖六款新增接入及四款既有游戏；检查范围、源码摘要与升级亮主／扣底补充记录见 `.local/gamepad/navigation-acceptance-manifest.json`。复跑使用同一 `check-browser.py`，通过 `--games gomoku,doudizhu,splendor,chess,xiangqi,weiqi,pairs,guandan,shengji,fortressblue` 指定范围，并使用新的输出目录。

赛车及共享输入回归见 `.local/gamepad/analog-regression/report.json`；该游戏完整规则、75 组自然完赛与 HTTP 证据位于 `.local/gamepad/racing-rules/`，竞态结果位于 `.local/gamepad/racing-race/`。浏览器延迟测试只延迟原始输入消息并保持 FIFO，不改写快照、规则或车辆位置。

强手棋、军棋、三国杀及共享导航回归见 `.local/gamepad/strategy-final/report.json`，四国军棋见 `.local/gamepad/junqi-four-controller/report.json`；检查范围与源码摘要见 `.local/gamepad/strategy-acceptance-manifest.json`。星垒技能悬停修复单独保留在 `.local/gamepad/starfort-hover-release-check/`，不代表该游戏已完成手柄接入。

地图导航回归见 `.local/gamepad/maps-final/report.json`，覆盖真实房间中的选地、派遣、建造、任务选择和铺路，以及 PC 改键、全屏、320px 手柄与原生触摸；探索者地图见 `.local/gamepad/catan-explorer-final/report.json`。源码摘要、检查结果和清理范围见 `.local/gamepad/maps-acceptance-manifest.json`。

征服时代与星垒的最终回归见 `.local/gamepad/conquest-starfort-final/report.json`；星垒接入控制器后的技能悬停复测见 `.local/gamepad/starfort-hover-controller-check/report.json`。检查范围、源码摘要与清理记录见 `.local/gamepad/conquest-starfort-acceptance-manifest.json`。

斯诺克最终验收见 `.local/gamepad/snooker-controller-final/report.json`，原生键盘到 Canvas 像素变化约 23 ms；最终公共导航回归见 `.local/gamepad/final-navigation-regression/report.json`。此数值来自本地验收环境，不代表所有设备的固定延迟。

## 最终验收

27 款平台注册游戏已接入。魂斗罗的操作先在隔离入口验收，发布前另补正式注册路径的控制器定义与 HTTP 配置读写验收，不改变游戏管理设置。27 款的操作证据、源码摘要和检查范围见 `.local/gamepad/final-acceptance.json`。发布与部署状态以对应版本记录为准。验收通过虚拟 Gamepad API 输入和原生键鼠／触屏完成，没有把任何具体手柄品牌或型号标记为实机认证。

公共检查保留原始结果及定向复核：默认预设测试曾在写入旧布局后复用过期修订号，触发预期的冲突保护；改为逐项读取最新配置后，连续 10 轮通过。应用的修订号校验未放宽。

临时前端构建、浏览器运行目录和重复结果已清理，保留最终证据及魂斗罗开发验收运行时。重新验收时，先运行以下命令生成隔离构建，再按报告中的 `--games` 范围执行浏览器脚本，并选择尚不存在的输出目录：

```sh
node web/node_modules/vite/bin/vite.js build --config scripts/controllers/gamepad/vite.config.mjs
python3 -B scripts/controllers/gamepad/check-browser.py --out .local/gamepad/recheck-snooker --games snooker
```
