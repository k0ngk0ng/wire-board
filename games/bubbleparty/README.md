# 泡泡大作战（bubbleparty）

受泡泡堂／Crazy Arcade 玩法启发的原创实时水泡对战：2／4／6／8 人分成红蓝两队，移动、放水泡、连锁爆破、破坏木箱、收集道具，把对方全员淘汰。素材全部原创，不使用商业游戏图片、角色或音效。

## 规则

- 15×13 对称竞技场；2／4／6／8 人，最多 4v4；前半座位红队、后半蓝队。
- 水泡约 2.2 秒后沿十字爆开，射程可被道具提升；水泡会阻挡移动（放置者离开原格后也会阻挡），并可与其它水泡连锁。
- 木箱被炸毁后可能掉落速度、水泡容量、射程、护盾；护盾抵挡一次爆炸。
- 全员淘汰的一方失败；3 分钟后安全区每 3 秒向内收缩一圈，区外角色淘汰。
- 时间到时按存活人数判定，人数相同为平台显式和局。
- 支持电脑玩家、托管/取消托管、观战、断线重连、刷新恢复和平台统一音效。

## 地图

`小区10`（默认，住宅小区主题）、`水乐园`、`森林公园`、`沙丘营地`，以及随机地图。所有地图 180° 对称、出生区无墙、出生区相连。

## 代码

```text
games/bubbleparty/
  server/     规则、实时帧、机器人、地图与状态校验
  web/        Canvas、输入、操作面板、封面与响应式样式
  tests/      规则单测、模块/HTTP E2E、JS 输入测试、浏览器夹具
  assets/     原创音频清单与说明
scripts/bubbleparty/
  prepare-audio.mjs   确定性原创 WAV 生成
  check-browser.py    真实浏览器验收
```

## 检查

```sh
. scripts/env.sh
go test -count=1 ./games/bubbleparty/... ./games
npm --prefix web test
npm --prefix web run build
python3 -B scripts/check_game_boundaries.py
python3 -B scripts/bubbleparty/check-browser.py --build
```

协议、失败场景和地图契约见 [CONTRACT.md](CONTRACT.md)，验收记录见 [docs/bubbleparty-verification.md](../../docs/bubbleparty-verification.md)。
