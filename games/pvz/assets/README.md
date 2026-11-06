# PC 初代完整资源

当前版本为 `pvz/classic-v2/`，保留旧 `classic-v1/`。完整目录包含原样本 53 个植物定义（49 个常规种子及 4 个小游戏模板）、33 种僵尸、6 个场景，以及割草机、拾取物、弹道、墓碑、梯子、冰道和原版受损图层。运行清单为 `public/pvz/classic-v2/manifest.json`，逐文件摘要为 `manifest-v2.json`。

v2 使用去重的原图层 WebP 和独立 `.pvzr` 载荷。`.pvzr` 是 gzip 压缩的 JSON，清单明确 `compression: "gzip"`、`format: "pvz-reanimation-json-v1"`，MIME 为 `application/octet-stream`，不是通用 CDN 的 `.gz` 旁车。所有公开路径使用安全字符；原始资源名含空格时采用 slug 和摘要后缀，矩阵引用同步更新。

解压后的片段保存每个原始姿态的图层、透明度及 Canvas 标准 `[a,b,c,d,e,f]` 矩阵。射手的身体／头部独立动画、三线与裂荚附属头、植物僵尸头、旗帜、僵王驾驶员和雪橇前后车壳均在合成数据中；植物受损图使用原始替换图片。`bounds` 保留完整有效图像边界，不能按普通格子裁掉跳跃、车辆或僵王。僵王使用场景坐标原点，蹦极携带植物使用原版独立垂直绘制偏移。

`byScene`／`byUnit` 负责场景和单位懒加载；`byPreview` 只包含独立小图，卡册不加载整套动作。相同原图层跨单位复用。生成过程曾产生约 271 MB 的逐帧中间图集，已在未发布的 v2 内清理并改为约 27 MB 的基础原图层数据；旧 v1 与原始研究源保留。

166 个原版 OGG 音效逐个与 PAK 内容比对，字节一致。经典音乐另由原始 MO3 离线渲染为 19 个无损 FLAC：菜单、选卡、草地、夜晚、泳池、浓雾、屋顶、Boss 与各分轨，并包含夜晚原曲高潮变奏。`music` 清单记录源 order、通道、PCM SHA-256、原前奏与循环采样帧。夜晚 order 92 是原版有意的静音待机段，已明确标记；不能以有声别曲替代。恢复范围、工具与重跑命令见[音乐取证](../../../docs/research/pvz/MUSIC.md)。

v2 从空目录重复生成时使用下面的流程。Pillow 环境建立方法见下方；BGM 解码依赖仅放在工作区研究缓存，不随游戏发布。

```sh
.local/pvz-research/venv/bin/python -B games/pvz/build/native_classic.py \
  .local/pvz-research/extracted \
  .local/pvz-rebuild/assets/public/pvz/classic-v2 \
  --audit .local/pvz-rebuild/native-audit.json
.local/pvz-research/venv/bin/python -B games/pvz/build/music_classic.py \
  --output .local/pvz-rebuild/assets/public/pvz/classic-v2 \
  --report .local/pvz-rebuild/music-render.json
.local/pvz-research/venv/bin/python -B games/pvz/build/verify_native.py \
  --root .local/pvz-rebuild/assets/public/pvz/classic-v2 \
  --output .local/pvz-rebuild/native-validation.json
.local/pvz-research/venv/bin/python -B games/pvz/build/verify_music.py \
  --root .local/pvz-rebuild/assets/public/pvz/classic-v2 \
  --output .local/pvz-rebuild/music-validation.json
```

## 保留的 v1 资源

v1 资源版本为 `pvz/classic-v1/`，来自归档的 PC 1.0.0.1051 `main.pak`。程序和原始包仅保存在工作区研究缓存，不作为浏览器发布资源。下载地址、样本身份和局限见 [取证报告](../../../docs/research/pvz/README.md)。这些图像和音效是原版资源，来源记录不表示新增或扩大其许可。

`public/pvz/classic-v1/manifest.json` 为游戏运行清单。`manifest.json` 为发布文件的大小和 SHA-256 清单。共有 43 段透明动画、1,239 帧、8 段原版音效，以及白天草坪、铲子、卡包、弹丸与阴影图片。

## 动画

`build/reanim.py` 解码原包的压缩 32 位重动画数据。定义尺寸 16 字节、轨道结构 12 字节、变换结构 44 字节；指针占位不被解释为资源地址，资源由后续字符串绑定。未指定字段沿用前一帧，首次默认位移／倾斜为 0、缩放／透明度为 1、图像帧为 0。

`build/render_classic.py` 将原版图层按轨道顺序和仿射变换合成为透明 WebP，使用 `lossless=True`；僵尸帽子及门、旗、泳圈等可选图层按种类过滤。保留全部选定源帧，并将源帧范围记入运行清单。默认播放率取文件声明，原版运行中的随机速率、附属动画混合与亚帧插值没有通过运行原版动态核验。

各单位的 `animations` 包含独立的 idle／walk／attack／death 等片段。每帧在图集中的位置为 `(index % columns * frameWidth, floor(index / columns) * frameHeight)`。`anchorX`／`anchorY` 是帧内脚底或植物格心锚点，战场应保持原版每格 80 × 100 像素的比例绘制；不能把撑杆跳或死亡动画的全部透明边界缩到一个格子中。单一单位所有片段使用相同边界，切换时锚点保持稳定。

原背景为 1,400 × 600，经典可视区取 `x=220, y=0, width=800, height=600`。该可视区内草坪格子区域为 `x=40, y=80, width=720, height=500`。

## 音频

8 段音效保留原版 OGG/Vorbis 字节，未进行有损转码。原始音频已经压缩，不追加 WAV 副本。`asset-validation.json` 保存 FFmpeg 解码后非空／非静音检查和 PCM16LE SHA-256。音频通过平台 `BoardProps.audio` 播放。

## 重复生成和验证

需要 Python、Pillow 和 FFmpeg。已验证的解释器为当前 worktree 内的 `.local/pvz-research/venv/bin/python`，版本为 CPython 3.14，Pillow 为 13.0.0；FFmpeg 使用本机已有命令。此 venv 通过 `python3 -m venv --without-pip` 创建，再从主工作区已有的 `.local/fortressblue-original/tools/py/PIL` 只读复制 Pillow，没有向外部目录安装依赖。

在本机重新建立该环境可执行以下命令。`../../.local/fortressblue-original/tools/py/PIL` 是从 `worktrees/pvz` 到主工作区已有依赖的相对路径，仅作为读取来源；全部新文件均写入当前 worktree：

```sh
python3 -m venv --without-pip .local/pvz-research/venv
python3 -B - <<'PY'
from pathlib import Path
import shutil
import subprocess
interpreter = '.local/pvz-research/venv/bin/python'
site = Path(subprocess.check_output(
    [interpreter, '-c', 'import sysconfig; print(sysconfig.get_path("purelib"))'],
    text=True,
).strip())
source = Path('../../.local/fortressblue-original/tools/py/PIL').resolve()
target = site / 'PIL'
if not target.exists():
    shutil.copytree(source, target, ignore=shutil.ignore_patterns('__pycache__', '*.pyc'))
PY
```

全部输入、输出和缓存均应保存在当前工作区内。先下载报告中固定摘要的 ZIP，再解出 `main.pak`，运行：

```sh
python3 -B -m unittest discover -s games/pvz/build -p 'test_*.py'
python3 -B games/pvz/build/extract_classic.py \
  .local/pvz-research/original-1.0.0.1051/main.pak \
  .local/pvz-research/extracted-new \
  --manifest .local/pvz-research/pak-members-new.json
.local/pvz-research/venv/bin/python -B games/pvz/build/render_classic.py \
  .local/pvz-research/extracted-new \
  .local/pvz-research/rendered-new \
  --contact .local/pvz-research/contact-new.png \
  --manifest .local/pvz-research/rendered-manifest-new.json
.local/pvz-research/venv/bin/python -B games/pvz/build/verify_classic.py \
  --output docs/research/pvz/evidence/asset-validation.json
```

v1 提取和生成命令拒绝已经存在的输出目录，防止覆盖；v2 原图层转换支持在本次生成目录内刷新清单与派生载荷，并清理其未引用中间文件。发布继续遵守仓库 OSS/CDN 规则，客户端图片、音频和 Wasm 不进入正式服务器包；线上上传、回读验证和剥离由统一发布流水线完成。
