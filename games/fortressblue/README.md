# 疯狂坦克（fortressblue）

2～10 人的回合制炮兵对战：红蓝两队各占一半席位，选择坦克、计算角度与力度、利用风向，炸毁地形并把对手打进深谷。最多 5v5，支持电脑玩家、托管、观战、断线恢复和手机触屏。

## 规则速览

- 人数：2／4／6／8／10，红队为前一半席位，蓝队为后一半；红蓝交替行动。
- 开局 20 秒选坦克，超时自动补齐；每回合默认 25 秒（可在创建房间时选择 15～40 秒）。
- 五种坦克：均衡（双联炮）、重装（重锤）、侦察（三连发）、狙击（穿甲弹）、支援（维修弹）。特殊弹数量有限。
- 行动：可多次移动（消耗移动力），调整角度（5～85°）、力度（20～100）和弹种后开火；一发炮弹结束本回合。
- 炮弹受重力和风力影响；爆炸会破坏地形。坦克落到新地面会受伤，落入深水会沉没。
- 8 分钟后进入突然死亡：伤害翻倍、风力增强；12 分钟按双方剩余总生命判定，相等为和局。
- 击毁对方全部坦克立即获胜；同一发炮弹双方全灭为和局。结算使用平台 `Team=true`，胜方全队进入 `Winners`。

完整协议和失败场景见 [CONTRACT.md](CONTRACT.md)。

## 代码结构

```text
games/fortressblue/
  server/       规则、地形、弹道、回合生命周期、电脑策略
  web/          Canvas 战场、选坦克、角度/力度/弹种控制、队伍 HUD
  tests/        模块全电脑对局、HTTP/SQLite、JS 控制测试
  assets/       原创封面、音效清单与本地回退音频
scripts/fortressblue/
  prepare-audio.mjs     确定性合成原创 WAV
  check-browser.py      真实浏览器验收
```

## 本地检查

```sh
go test -count=1 ./games/fortressblue/... ./games
npm --prefix web run build
npm --prefix web test -- fortressblue
python3 -B scripts/check_game_boundaries.py
python3 -B scripts/fortressblue/check-browser.py
```

## 素材

参考游戏仅用于玩法灵感。本项目无法在本机取得/运行参考游戏，因此没有复制任何参考游戏图片、音频或名称：
封面是手绘式原创 SVG，坦克与地形由 Canvas 实时绘制，音效和战斗音乐由
`scripts/fortressblue/prepare-audio.mjs` 离线合成。来源、生成方式和 SHA-256 见
`assets/README.md` 与 `assets/manifest.json`。游戏默认下架，发布不会自动上架。
