# 图标与音效来源

正式配对素材为 51 个经典 QQ 连连看图案，未使用 Emoji 或其他图标代替。

- 获取来源：[allfornaruto/ts-kyodai](https://github.com/allfornaruto/ts-kyodai)，一个公开的 QQ 连连看复刻项目。
- 固定版本：`4990ba00e7b796a5a366f7cea78c3bfa8248c15c`。
- 图标：上游 `assets/img/block/icon/1.png` 至 `51.png`，保留原始文件。
- 闪电：上游 `assets/img/lightning_x/lightning_x_1.png` 至 `5.png`，配合 Canvas 绘制合法连接路径、端点光环和粒子。
- 音效：上游 `assets/resources/mp3/` 中的选择、消除、开始、结束、破障和背景音乐素材；使用 Web Audio 与 HTML Audio 播放。
- 每个素材的源地址、文件大小和 SHA-256 校验值见 `public/assets/manifest.json`。
- `audio/effects-*.js` 为上述 5 个原始 WAV 的 Base64 打包，无音色修改；由 `scripts/build-audio-bank.mjs` 生成，用于 CDN 预加载和 Web Audio 内存播放。文件名包含内容 SHA-256 前 12 位。
- [图标总览](docs/previews/icons.png)。

视觉核对参考：[QQ 连连看客户端截图](https://github.com/gyk001/QQ_LLK_Cheat/blob/master/raw/llk_demo.png)。企鹅、动物、星星、手势、彩球等图案与参考截图一致。来源为公开复刻项目，不声称这些文件是腾讯官方发布的资源包；音效使用该项目提供的经典素材，尚未通过官方原始包做二进制一致性验证。

背景、障碍物、按钮、布局、登录页和 Logo 在本项目中以 CSS / SVG 实现；界面线性图标使用 Lucide。游戏运行时不依赖外部图片或字体服务。

当前背景配乐 `audio/bg-legacy-2015.mp3` 来自 [sexdevil/lianliankan](https://github.com/sexdevil/lianliankan) 的 2015 年提交 `18d9111ebc4b6e77dc31bdb9e45c65fb00659111`，同仓库 `sound/bg.mid` 的曲名元数据为「找对子」。用户于 2026-10-05 试听确认使用这首配乐，自 v1.1.2 起固定为游戏默认背景音乐。未认证为腾讯官方客户端原始文件。原 `bg.mp3` 保留来源记录，但不再播放。
