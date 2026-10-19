# 泡泡大作战素材

本目录只包含项目原创内容。角色、地图、封面和音效均为原创，不使用泡泡堂／Crazy Arcade 的商业图片、角色名称、音频或原始地图素材。

## 音效

`public/bubbleparty/v1/audio/` 由 `scripts/bubbleparty/prepare-audio.mjs` 确定性地离线合成：

```sh
node scripts/bubbleparty/prepare-audio.mjs
```

- 作者／许可：wire-board contributors，项目原创。
- 生成方式：正弦波、方波、滑音和噪声包络合成，44.1 kHz、16 位、单声道 PCM WAV。
- 文件：`countdown.wav`、`start.wav`、`place.wav`、`explode.wav`、`item.wav`、`shield.wav`、`eliminate.wav`、`sudden.wav`、`win.wav`、`lose.wav`、`battle.wav`（8 秒循环）。
- `manifest.json` 记录生成器 SHA-256、每个文件的字节数、相对路径和 SHA-256。
- 音效只通过平台 `BoardProps.audio` 播放，游戏不创建播放器或 `AudioContext`。

## 封面

`web/Cover.tsx` 中的手绘式矢量 SVG 为原创：红蓝两队角色、水泡和小区房屋，横向、低饱和；大厅、创建与等待房间共用同一封面。

## 地图

`小区10`、`水乐园`、`森林公园`、`沙丘营地` 都是原创对称竞技场，不复制商业游戏地图；`小区10` 使用住宅小区主题的地面、房屋墙块和道路装饰。
