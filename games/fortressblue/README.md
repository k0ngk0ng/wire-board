# 疯狂坦克（fortressblue）

2／4／6／8 人的回合制坦克炮战，复刻《Fortress 2 BLUE》的 17 张原版地图、14 种坦克、
可破坏像素地形、风向与特殊弹。红蓝两队各占一半席位，最多 4v4；支持电脑玩家、托管、
观战、断线恢复、PC 键盘和手机触屏。

## 规则速览

- 人数：2／4／6／8，红队为前一半席位，蓝队为后一半；红蓝交替行动。
- 开局 20 秒选坦克，超时自动补齐；每回合默认 30 秒（创建房间时可选 15～40 秒）。
- 14 种原版坦克，属性来自官方大厅配置；主炮无限，特殊弹每局 2 发，附带爆炸/毒/火/雷效果。
- 行动：可多次移动（消耗移动力），调整角度（0～坦克上限）、力度（20～100）、朝向和弹种后开火；
  一发炮弹结束本回合。
- 炮弹受重力和风力影响；爆炸永久破坏原版地图前景层。坦克坠落会受伤，掉出地图会阵亡。
- 8 分钟后进入突然死亡：伤害翻倍、风力增强；12 分钟按双方剩余总生命判定，相等为和局。
- 击毁对方全部坦克立即获胜；同一发炮弹双方全灭为和局。结算使用平台 `Team=true`，胜方全队进入 `Winners`。

## 操作

- PC：鼠标指向战场即可瞄准，角度随指针变化，滚轮调力度，单击战场开火；也可用
  ←／→ 或 A／D 移动、↑／↓ 或 W／S 调角度、Z／X 调力度、Q／E 切换弹种、空格开火。
- 手机：使用操作面板的角度／力度滑杆、左右移动、朝向、弹种与开火按钮。
- 地图在创建房间时选择；在等待房间修改地图或规则后需先点「保存设置」，未保存时不能开始。
- 战场按 2.4× 原画绘制坦克并跟随当前坦克取景，绘制与服务器一致的落点预览、屏幕边缘敌人
  指示和小地图；桌面与手机均有可辨识的坦克轮廓和玩家颜色。

完整协议和失败场景见 [CONTRACT.md](CONTRACT.md)。

## 代码结构

```text
games/fortressblue/
  server/       规则、位图地形、弹道、回合生命周期、电脑策略
  maps/         原版 17 张地图目录与嵌入的碰撞位图
  data/         原版坦克属性、技能与媒体路径
  web/          Canvas 战场、选坦克、角度/力度/弹种控制、队伍 HUD
  tests/        全电脑对局、HTTP/SQLite、选坦克、重连
  assets/       来源、生成器、SHA-256 清单与本地回退媒体（Git 忽略）
scripts/fortressblue/
  prepare-original-assets.py   从本机已获取的官方网页客户端资源生成媒体包
  verify-assets.py             本地/dist/CDN 字节与媒体校验
  check-browser.py             真实浏览器验收
```

## 本地检查

```sh
go test -count=1 ./games/fortressblue/...
npm --prefix web run build
python3 -B scripts/check_game_boundaries.py
python3 -B scripts/fortressblue/verify-assets.py --dist
python3 -B scripts/fortressblue/check-browser.py
```

## 素材

原版坦克、地图、炮弹、爆炸和音乐来自官方网页客户端的公开资源包，版权归 CCR / 原权利方所有，
仅经用户明确授权在本私有平台使用；原始媒体不提交 Git。解码链为 Emscripten 包拆分 →
LH5 解压 → 逐行 span RLE；地图碰撞由原版前景层 alpha 生成。来源、方法、许可与逐文件
SHA-256 见 [assets/README.md](assets/README.md)、`assets/sources.json` 和 `assets/manifest.json`。
封面为本项目原创手绘 SVG。游戏默认上架；管理员可随时下架。
