# 坦克大战（battlecity）

服务端权威的 1v1／2v2／3v3 实时坦克对战。两支队伍各有大本营；敌方炮弹命中大本营即获胜。支持原版电脑坦克、随机道具、12 张双基地地图、电脑玩家、托管/取消托管、观战、断线重连和恢复存档。

## 规则速览

- 人数：2／4／6，两队人数相等；座位 0～`teamSize-1` 为上方，其余为下方。
- 阶段：选坦克 15 秒 → 倒计时 3 秒 → 对战最多 10 分钟。
- 大本营一击即毁；坦克阵亡 4 秒后重生，出生保护 2 秒；技术等级阵亡清零。
- 5 种坦克：均衡、基础、快速、火力、装甲；开局选择，双方可重复。
- 击毁敌方坦克约 30% 概率掉随机道具：星、手雷、头盔、铲子、备用坦克、时钟。
- 12 张 180° 对称地图；房主创建/等待时选择，另有随机地图。
- 和局使用平台显式 `Draw`；含电脑对局由平台不计分。

完整协议、失败场景和素材契约见 [CONTRACT.md](CONTRACT.md)。

## 代码结构

```text
games/battlecity/
  server/      Go 规则、100ms 帧/20ms 子步、六席生命周期、电脑策略
  maps/        共享 catalog.json + Go embed/校验
  web/         Canvas、选坦克、HUD、键盘/触屏、平台音频
  tests/       模块 E2E、HTTP、JS、真实浏览器夹具
  assets/      manifest/sources/labels/SVG 封面；商业媒体不入 Git
scripts/battlecity/
  generate_maps.py       生成/校验 12 张地图
  extract_static.py      CHR/静态调色板
  capture_probe.py       FCEUX 参考截图/PPU/OAM
  capture_sound.py       声音队列/APU 追踪
  check-browser.py       真实浏览器验收
```

## 本地检查

```sh
. scripts/env.sh
go test -count=1 ./games/battlecity/...
npm --prefix web test -- battlecity
npm --prefix web run build
python3 -B scripts/check_game_boundaries.py
python3 -B scripts/battlecity/generate_maps.py
python3 -B scripts/battlecity/check-browser.py --build
```

## 素材

源 ROM 为用户提供的 `Battle City (J).nes`（SHA-256 `a869aead…f220c0c`）。派生商业媒体只放 `.local` 与 OSS 不可变前缀 `bga/v20261002-1/battlecity/v1/`，不提交 Git。素材提取、校验和上传说明见 `assets/README.md` 与 `docs/battlecity-verification.md`。

游戏默认上架；管理员可在后台显式下架，部署/发布保留设置。
