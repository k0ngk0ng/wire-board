# 俄罗斯方块素材

本目录只包含项目原创内容：封面是 `art.ts` 中的手绘式矢量 SVG，音效由
`scripts/tetris/prepare-audio.mjs` 离线合成，没有使用外部采样、盒装照片或扫描封面。
成品位于 `public/tetris/v1/audio/`，随前端构建进入 `web/dist/tetris/v1/audio/`；
部署时可选上传到既有素材基地址下的 `tetris/v1/`，前端始终保留本地回退。

## 生成与许可

```sh
node scripts/tetris/prepare-audio.mjs
python3 -B scripts/tetris/verify-assets.py --dist
```

- 作者／许可：wire-board contributors，项目原创。
- 生成方式：确定性的正弦／方波／噪声包络合成，44.1 kHz、16 位、单声道 PCM WAV。
- `manifest.json` 记录生成器 SHA-256、每个文件的路径、字节数和 SHA-256。
- 音效只通过平台 `BoardProps.audio` 播放；游戏不创建播放器或 `AudioContext`。

## 文件

| 文件 | 用途 |
| --- | --- |
| `start.wav` | 倒计时结束、开始落块 |
| `lock.wav` | 方块锁定 |
| `clear.wav` | 普通消行 |
| `tetris.wav` | 一次消除四行 |
| `garbage.wav` | 垃圾行落位 |
| `level.wav` | 升级 |
| `eliminate.wav` | 本席出局 |
| `win.wav` | 本席获胜 |
| `finish.wav` | 观战或非本席获胜的对局结束 |

修改音效后重新运行生成与校验脚本，并保留新的 SHA-256 清单；已发布到 OSS 的
旧文件不得覆盖，应使用新的版本目录。当前实现未执行 OSS 上传，本地回退完整可用。
