# 疯狂坦克素材

本目录记录原版《Fortress 2 BLUE》媒体包与其来源、校验与发布方式。原始媒体位于
`assets/public/fortressblue/v2/`（Git 忽略），不提交到仓库。

## 来源与许可

- 来源：官方网页客户端 `https://game.fortress2blue.com/` 的公开资源包
  `tankclient_emsc.c906c.data.00-02`、`Download/` 资源与 Cocos 大厅配置。
- 获取日期：2026-10-20。
- 版权：归 CCR / 原权利方所有。仅经用户明确授权用于本私有平台；不对外再分发。
- 原始包 SHA-256 与提取方法见 `sources.json`；v2 媒体通过 OSS 不可变目录发布，v3 小型动画图集随源码保存。
- 封面仍为本项目原创手绘 SVG，未使用原版盒装封面或标题画面。

## 解码链

1. 拆开 Emscripten file package（`tankclient_emsc.c906c.data.00-02`）。
2. `.i16` 帧数据使用 **LH5（LHA）** 解压。
3. 类型 0 帧再按逐行 span RLE 解出 16-bit RGB565；透明色 `#F81F`。
4. 地图前景层的 alpha 生成 4×4 px 权威碰撞位图；原始图用于客户端渲染与弹坑擦除。
5. 原版 MIDI 使用包内 `timidity.cfg` / `instruments` 渲染；MP3 仅作游戏播放。

## 恢复与检查

```sh
python3 -B scripts/fortressblue/fetch-assets.py
python3 -B scripts/fortressblue/verify-assets.py --dist
```

v2 为已发布的不可变媒体。旧生成器只供隔离来源验证调用，不再覆盖现有资源或当前游戏规则。已发布清单包含：

- `assets/public/fortressblue/v2/maps/`：17 张地图背景、前景、缩略图和音乐。
- `assets/public/fortressblue/v2/tanks/`：14 种坦克的 idle/fire/die/move 与立绘。
- `assets/public/fortressblue/v2/projectiles/`、`effects/explosion.webp`。
- `assets/public/fortressblue/v2/audio/`：原版音效与地图音乐。
- `maps/catalog.json`（服务端含碰撞位图）、`maps/client.json`（前端轻量目录）。
- `data/tanks.json`（坦克属性、技能、媒体路径）。
- `assets/manifest.json`（186 个媒体文件的 SHA-256、字节数与 CDN 路径）。

## 发布

```sh
python3 -B scripts/upload_assets.py games/fortressblue/assets/public/fortressblue/v2 \
  --include-audio --bucket stoneage-assets \
  --endpoint stoneage-assets.oss-cn-shanghai.aliyuncs.com \
  --prefix bga/v20261002-1/fortressblue/v2
python3 -B scripts/fortressblue/verify-assets.py --dist --oss
```

不可变前缀为 `bga/v20261002-1/fortressblue/v2/`；修改素材时使用新版本目录并更新清单，
不得覆盖已发布对象。前端优先 CDN，失败时回退到同源 `/fortressblue/v2/` 本地副本。

## v3 原生动画

坦克动画重构与验证见 [README-v3.md](README-v3.md)。v2 已发布文件保持不变。
