# 泡泡堂素材来源

当前地图／水泡使用 `public/bubbleparty/v2/`，可选人物使用 `public/bubbleparty/v3/avatars/`。v2 图片来自第三方网页 Demo；v3 人物和音频来自已提取的 2002 年官方客户端。来源分别记录，不能将第三方图集称为原包提取图。原作素材的再发布授权尚未核实，仓库记录该状态；v2 和 v3 已于 2026-10-21 按清单上传至现有 CDN 的不可变版本路径，并逐对象核对 SHA-256。

## v2 图片

- 来源：https://github.com/visolleon/bnb
- 固定提交：`7070051995a5feab0ca83eabc708cdecda1e0e74`。
- ZIP SHA-256：`536ca0f8f7f05faf88fddeddc37cda6e676d75013e126fd2a5e59e6109ed24b4`。
- 选取 `Pic/` 下 21 张原 PNG，逐字节复制到 `v2/sprites/`，没有重新编码或覆盖旧版本。
- 图集包括两角色四向步态、困泡／死亡、水泡、水柱、小区地形及容量／速度／射程道具。原 Demo 默认小区 13；本项目小区 10 地形依据另存的官方截图转录，不复制 Demo 的地图规则。
- 原仓库没有 LICENSE 文件，其 README 没有提供原始商业素材授权。保留来源事实，未标注为开源美术。
- `web/sprites.ts` 记录每帧尺寸和裁切；异步失败顺序回退同源图，无图时 Canvas 绘制可辨认的矢量地形和角色。

## v2 音频

- 存档来源：https://archive.org/download/crazy_arcade_setups/2002.03.13./Crazy%20Arcade.zip
- 原 ZIP SHA-256：`326c9b89b17e3404e192073261f54f34e27e5225d3f955e39bc8d97204405ae6`。
- 使用 REA 检查并解包后，`scripts/bubbleparty/extract-classic-audio.py` 读取 `Fx.idx` 中的名字／偏移／长度，从 `Fx.idd` 提取标准 WAV／MP3。数据块保持原字节；发布文件只增加正确扩展名。
- 本目录选取小区音乐 `Village.mp3`，以及放泡、爆炸、困泡、救援、死亡、拾取、胜／负／平、开始、倒计时和时间耗尽声音，共 13 个文件。
- 不将 `Wave/` 标识或文件名本身当作原版运行行为的证明。平台事件映射是本项目的实现选择，音频一律交给 `BoardProps.audio` 播放。

`manifest-v2.json` 记录每文件 URL、原资源标识、索引偏移、源 SHA-256、输出 SHA-256、字节数和复制方式。图集尺寸额外记录在 `dimensions`，音频绑定原 `Fx.idx`／`Fx.idd` 摘要。

```sh
python3 scripts/bubbleparty/check-assets.py
node --test games/bubbleparty/tests/sprites.test.mjs
```

## v1 历史资产

`public/bubbleparty/v1/audio/` 和 `manifest.json` 保留旧版的项目合成音效与生成器摘要。本次没有改变 v1 字节。原先「所有角色、地图和音频均为原创」的说明不再适用于 v2。封面沿用平台的 SVG 插画规范，与对局图集分开维护。

## v3 可选人物

4 套人物来自 `Fx.idd` 的 `Character/C00`～`C03`，每套含四向行走、眨眼、困泡与死亡引用帧。使用中性外观名称「泡泡帽／飞行帽／熊耳帽／条纹帽」，没有把开发编号画到人物上。人物外观与阵营分离，红蓝两队各有完整 4 套。

`prepare-avatars.py` 校验原 `Fx.idx/Fx.idd` SHA，按索引解析 RGB565 RLE 和真实帧矩形，并从 ObjectTable 读取帧序列；不执行客户端。Static 先铺底，再由 Adjust 覆盖，蓝队仅交换 Adjust 层 R/B 通道。C00 右向 Static 本身含红衣像素，逆序合成会使蓝队右转变红，已用全状态逐像素核查排除。命名、队色变换、显示时基（原始权重每单位按 20 ms 展示）和地面锚点是本项目接入选择，不冒称原引擎已验证参数。

生成 48 张图集，源偏移、源层 SHA、原始帧字段与输出 SHA 保存在 `manifest-v3.json`；前端使用 `avatars-v3.json`。困泡引用 Die 前两帧，死亡图集中有不同高度的帧，必须按矩形取样。脸部由原始非着色层保留；读取失败时回退为同队色、可辨造型的 Canvas 人物。

```sh
python3 -B scripts/bubbleparty/prepare-avatars.py --client-dir .local/bubbleparty-original/client-2002
python3 -B scripts/bubbleparty/check-assets.py
```

图像仍采用 CDN 不可变 v3 路径与同源本地回退；48 张 v3 图集及 34 份 v2 素材已上传并核对 OSS／CDN 字节；本地回退同时保留，生产版本以 GitHub Release 记录为准。原包授权状态沿用来源清单，未改写为原创或已获再发布许可。
