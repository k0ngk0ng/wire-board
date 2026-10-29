# 开发守则索引

开发前阅读[通用规范](rules/common.md)，再按改动范围阅读专题规范和对应游戏文件。通用要求不在各游戏文件中重复；新增游戏时，补充 `rules/<game-id>.md` 并更新本索引。

## 通用与专题规范

| 规范 | 适用范围 |
| --- | --- |
| [通用规范](rules/common.md) | 所有开发：测试、工作区、检查发布、游戏接入、积分和电脑玩家 |
| [即时游戏开发规范](rules/realtime.md) | 即时操作、本地预测、服务器校正、慢连接隔离和响应延迟验收 |
| [扑克牌游戏开发规范](rules/poker.md) | 选牌、牌面、手牌布局、牌桌稳定性和手机交互 |
| [复刻游戏开发规范](rules/recreation.md) | 原版对照、独立规则实现、平台席位与原版角色映射 |
| [界面与交互开发规范](rules/ui.md) | 公共界面、视觉、动画、全屏和游戏目录 |
| [控制器开发规范](rules/controllers.md) | 键鼠、触屏、控制器映射、连发和输入释放 |
| [加载与资源发布规范](rules/loading.md) | 按需加载、就绪门槛、OSS/CDN 资源和构建剥离 |
| [音效开发规范](rules/audio.md) | 平台音频接口、静音、媒体播放和音频恢复 |
| [地图交互开发规范](rules/maps.md) | 具有缩放功能的地图、拖动平移、命中与视区比例 |
| [塔防游戏开发规范](rules/tower-defense.md) | 建造点、塔身操作、缩放命中与手机交互 |

## 游戏专属规范

文件名与 `games/` 下的游戏 ID 一致。

| 游戏 | 规范文件 |
| --- | --- |
| 热血篮球 | [basketball.md](rules/basketball.md) |
| 坦克大战 | [battlecity.md](rules/battlecity.md) |
| 泡泡堂 | [bubbleparty.md](rules/bubbleparty.md) |
| 卡卡颂 | [carcassonne.md](rules/carcassonne.md) |
| 卡坦岛 | [catan.md](rules/catan.md) |
| 国际象棋 | [chess.md](rules/chess.md) |
| 征服时代 | [conquest.md](rules/conquest.md) |
| 魂斗罗 | [contra.md](rules/contra.md) |
| 斗地主 | [doudizhu.md](rules/doudizhu.md) |
| 疯狂坦克 | [fortressblue.md](rules/fortressblue.md) |
| 五子棋 | [gomoku.md](rules/gomoku.md) |
| 掼蛋 | [guandan.md](rules/guandan.md) |
| 军棋 | [junqi.md](rules/junqi.md) |
| 强手棋 | [monopoly.md](rules/monopoly.md) |
| 连连看 | [pairs.md](rules/pairs.md) |
| 极品飞车 | [racing.md](rules/racing.md) |
| 铁路环游 | [rail.md](rules/rail.md) |
| 三国杀 | [sanguosha.md](rules/sanguosha.md) |
| 升级 | [shengji.md](rules/shengji.md) |
| 斯诺克 | [snooker.md](rules/snooker.md) |
| 街霸方块 | [spf2t.md](rules/spf2t.md) |
| 璀璨宝石 | [splendor.md](rules/splendor.md) |
| 星垒 | [starfort.md](rules/starfort.md) |
| 乒乓球 | [tabletennis.md](rules/tabletennis.md) |
| 俄罗斯方块 | [tetris.md](rules/tetris.md) |
| 围棋 | [weiqi.md](rules/weiqi.md) |
| 中国象棋 | [xiangqi.md](rules/xiangqi.md) |
