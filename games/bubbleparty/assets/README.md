# 泡泡堂素材来源

当前对局使用 `public/bubbleparty/v2/`。图片来自已在本地运行的第三方网页还原 Demo；音频来自存档中的 2002 年官方客户端。两类来源分开记录，不能将第三方图集称为本项目原创或本次从官方客户端提取的图片。原作素材的再发布授权尚未核实，仓库记录该状态；本次没有向 OSS 上传。

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
