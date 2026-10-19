# 疯狂坦克素材

本目录只包含项目原创内容。参考游戏仅用于玩法灵感；本机不能取得/运行参考游戏，因此没有复制参考游戏的图片、音频、名称或盒装封面。

## 生成

```sh
node scripts/fortressblue/prepare-audio.mjs
python3 -B scripts/fortressblue/verify-assets.py --dist
```

- 作者／许可：wire-board contributors，项目原创。
- 音效：确定性正弦／方波／噪声包络合成，44.1 kHz、16 位、单声道 PCM WAV。
- 封面和坦克：`web/Cover.tsx` 的 SVG 与 `web/Board.tsx` 的 Canvas 矢量绘制，不依赖外部图片。
- `manifest.json` 记录生成器 SHA-256、每个文件的相对路径、字节数和 SHA-256。

## 文件

| 文件 | 用途 |
| --- | --- |
| `select.wav` | 选择坦克 |
| `shot.wav` | 火炮发射 |
| `explode.wav` | 炮弹落地/命中爆炸 |
| `hit.wav` | 造成伤害或治疗 |
| `death.wav` | 坦克被击毁/沉没 |
| `turn.wav` | 轮到下一辆坦克 |
| `sudden.wav` | 进入突然死亡 |
| `win.wav` / `lose.wav` | 结算 |
| `battle.wav` | 8 秒原创战斗循环音乐 |

本地回退位于 `public/fortressblue/v1/audio/` 并随前端构建进入 `web/dist/`；配置
`ASSETS_BASE_URL` 时优先从 `${ASSETS_BASE_URL}/fortressblue/v1/audio/` 加载。音效只通过平台
`BoardProps.audio` 播放，游戏不创建播放器或 `AudioContext`。

可选上传（前缀不可覆盖已有对象）：

```sh
python3 -B scripts/upload_assets.py games/fortressblue/assets/public/fortressblue/v1 \
  --include-audio --bucket stoneage-assets \
  --endpoint stoneage-assets.oss-cn-shanghai.aliyuncs.com \
  --prefix bga/v20261002-1/fortressblue/v1
```

已发布到 OSS 的文件不得覆盖；修改素材时使用新的版本目录并保留新的 SHA-256 清单。
