# 巡径守卫素材

素材参考来源为 [PokéPath TD v1.7.3](https://pokepath-game.pages.dev/)，原网页入口为 <https://pokepathtd.org/>。`public/pokepath/v1/` 仅提供图形、音频、字体和规范化 JSON 事实；原版程序与样式表留在研究目录，不进入产品载荷。界面和规则由本项目自行实现。

[`manifest.json`](manifest.json) 记录素材来源 URL、发布路径、字节数、SHA-256、MIME 和图片尺寸。图片、字体和原 MP3 保持来源字节；明确记录的缺失图标派生例外见 [`source-defects.md`](source-defects.md)。`referenceOnlyModules` 只记录研究模块身份，不发布模块正文。

72 个源 WAV 中，70 个整数 PCM 使用 FLAC 无损压缩；`sword.wav` 与 `ui/reroll.wav` 是 IEEE float32，保留兼容 WAV，不能量化成整数后声称无损。音效总字节从 8,980,139 降至 3,155,493，节省 5,824,646 字节。每个音频条目的 `audio` 保存逻辑路径、源字节身份、采样率、声道、位深、采样帧数、解码 PCM SHA-256、变换原因和压缩比。原始 WAV 保存在 `.local/pokepath/research/audio-originals/`，已压缩的 70 个原文件不再进入发布目录。

`resources.json` 的 `aliases` 将原逻辑 WAV 路径映射到实际 FLAC 路径；文件大小、摘要和 MIME 始终属于实际发布对象。逻辑声音 key 与游戏规则不改，平台播放器读取别名解析后的原生音频 URL。极短文件若 FLAC 不更小，压缩器保留源 WAV 并记录候选实际体积。

所有发布路径必须符合公共 CDN 的规范字符规则。原文件 `marshadow-walk (2).png` 与 `marshadow-walk.png` 字节完全相同；前者发布为 `marshadow-walk-2.png`，清单的 `sourcePath`、原 URL 和摘要保留来源。重建器拒绝空格、括号及路径穿越，不能通过放宽公共校验恢复原文件名。

规范化目录与资源索引可在仓库根目录重建并验证：

```sh
node games/pokepath/build/compress-audio.mjs
node games/pokepath/build/extract-reference.mjs
node --test games/pokepath/tests/audio-lossless.test.mjs games/pokepath/tests/native-reference.test.mjs
```

压缩器使用现有 FLAC 1.5.0 编码器与 FFmpeg／ffprobe 9.0.1 验证全部样本；8 位 WAV 必须保持 8 位有效精度，不能使用升为 16 位的候选冒充相同格式。缓存缺失时，压缩器可按固定来源 URL 恢复原 WAV，核对源字节摘要后只写入研究缓存。重复运行校验并复用已有产物，不恢复发布目录中的原 WAV。数值重建器只读取已捕获的研究字面表及本目录清单，不执行原版程序。

正常构建直接使用已经生成的媒体和事实目录，不要求重新下载研究素材。正式资源沿用平台 OSS/CDN 上传去重、回读校验和服务器构建剥离流程；按当前地图、队伍和曲目下载媒体正文。无损验收报告位于 `.local/pokepath/verification/audio-lossless/`。

上游素材署名包括宝可梦相关权利人、PMD Collab、PokéSprite、BDRAGON1727、Kiranshastry、Freepik、Media Factory、Sokudo 及原版列出的其他贡献者。完整来源和权利说明见 [原版研究](../../../docs/research/pokepath/README.md)及平台 [第三方清单](../../../docs/THIRD_PARTY.md)。
