# PC 初代资源

资源版本为 `pvz/classic-v1/`，来自归档的 PC 1.0.0.1051 `main.pak`。程序和原始包仅保存在工作区研究缓存，不作为浏览器发布资源。下载地址、样本身份和局限见 [取证报告](../../../docs/research/pvz/README.md)。这些图像和音效是原版资源，来源记录不表示新增或扩大其许可。

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

提取和生成命令拒绝已经存在的输出目录，防止覆盖。发布继续遵守仓库 OSS/CDN 规则，客户端图片、音频和 Wasm 不进入正式服务器包。本轮仅生成本地资源，尚未上传 OSS。
