# 游戏生图技巧

本文记录「密室逃脱」美术素材的实际制作流程，供后续游戏复用。覆盖当前 Codex 所配置 API 的调用、风格统一、行走图集、素材处理与验收。资源接入和发布仍遵循[加载与资源发布规范](../rules/loading.md)及[界面与交互开发规范](../rules/ui.md)。

## 模型与调用方式

本次经用户授权，通过当前配置的 API 使用 `gpt-image-2.5`，生成与编辑均已实际使用。执行工具为已安装 imagegen Skill 自带的 `scripts/image_gen.py`，没有修改 Skill，也没有另写 SDK 生图程序。

这是本次明确指定 API 和模型后的调用经验。后续任务根据当时的 Skill 说明选择工具；普通生图优先使用内置工具，指定 API 或模型时采用对应调用方式。

| 用途 | CLI 子命令 | 当前兼容接口（相对于 `base_url`） |
| --- | --- | --- |
| 单张生成 | `generate` | `POST /images/generations` |
| 多个不同素材 | `generate-batch` | 并发调用生成接口，每个 JSONL 行一个任务 |
| 根据参考图编辑 | `edit` | `POST /images/edits`，上传参考图 |

当前配置的 `base_url` 已包含 `/v1`，拼接接口时不要重复添加。CLI 负责请求和解码返回图片；主要参数为 `model`、`prompt`、`size`、`quality`，编辑额外传入图片。

本次在配置服务的 `GET {base_url}/models` 中查到 `gpt-image-2.5`、`gpt-image-2.5-sunburst`、`gpt-image-2.5-flare`、`gpt-image-2` 和 `gpt-image-1.5`。只实际验证了 `gpt-image-2.5`，没有对其他变体做质量或性能比较。模型列表由当前服务提供，不据此推断所有环境都可使用这些模型。

### 准备仓库内环境

从仓库根目录执行，使用 Python 3.11 或更新版本。先加载仓库环境，再创建虚拟环境和安装依赖，使临时文件与缓存留在工作区内。

```bash
. scripts/env.sh
export PIP_CACHE_DIR="$PWD/.local/pip-cache"
export PYTHONDONTWRITEBYTECODE=1
python3 -m venv .local/imagegen-venv
.local/imagegen-venv/bin/python -m pip install openai Pillow
export IMAGEGEN_PYTHON="$PWD/.local/imagegen-venv/bin/python"
export IMAGE_GEN="$HOME/.codex/skills/.system/imagegen/scripts/image_gen.py"
```

`IMAGE_GEN` 是本次安装位置的通用写法；安装路径变化时，以会话技能目录中 imagegen 的实际位置为准。只读取、执行 Skill 文件，不向 Skill 目录安装依赖或写入缓存。

### 将当前配置传给 CLI

当前环境的服务配置位于 Codex 配置文件：通过 `model_provider` 选择 `model_providers` 中的条目，读取 `base_url` 和认证信息。本次认证来自 `experimental_bearer_token`；其他环境也可能通过 `env_key` 指定令牌环境变量。

以下函数仅在 Python 子进程内设置 `OPENAI_BASE_URL` 和 `OPENAI_API_KEY`，再执行原有 CLI。也支持提前设置这两个环境变量，但必须成对设置，避免把一个服务的令牌发给另一个服务。不要把实际地址、令牌或完整配置复制进仓库、聊天、命令参数和日志。

```bash
imagegen() {
  "$IMAGEGEN_PYTHON" -B - "$@" <<'PY'
import os
import runpy
import sys
import tomllib
from pathlib import Path

base_url = os.environ.get("OPENAI_BASE_URL")
api_key = os.environ.get("OPENAI_API_KEY")
if bool(base_url) != bool(api_key):
    raise SystemExit("请成对设置 OPENAI_BASE_URL 和 OPENAI_API_KEY，或同时取消设置")
if not base_url:
    codex_dir = Path(os.environ.get("CODEX_HOME", str(Path.home() / ".codex")))
    config = tomllib.loads((codex_dir / "config.toml").read_text())
    provider = config["model_providers"][config["model_provider"]]
    base_url = provider.get("base_url")
    env_key = provider.get("env_key")
    api_key = (os.environ.get(env_key) if env_key else None)
    api_key = api_key or provider.get("experimental_bearer_token")
if not base_url or not api_key:
    raise SystemExit("当前配置缺少 API 地址或令牌；请在本地配置，不要输出密钥")

os.environ["OPENAI_BASE_URL"] = base_url.rstrip("/")
os.environ["OPENAI_API_KEY"] = api_key
sys.argv = [os.environ["IMAGE_GEN"], *sys.argv[1:]]
runpy.run_path(os.environ["IMAGE_GEN"], run_name="__main__")
PY
}
```

此示例不会修改 Codex 配置，也不处理其他登录方式；读取不到所需字段时停止，按所在环境配置认证。`-B` 防止在外部 Skill 目录产生 `__pycache__`。

### 单张生成与批量生成

先用 `--dry-run` 检查参数和输出路径；去掉该参数才会调用计费 API。每次迭代使用新的输出文件名，保留已批准版本。

```bash
imagegen generate \
  --model gpt-image-2.5 \
  --prompt "Original oblique top-down game room, teal and brass palette, warm upper-left light, clear central walkable floor, no characters, no text or watermark" \
  --size 1536x1024 --quality high \
  --out output/imagegen/station-draft-v2.png \
  --dry-run
```

本次六个初始素材的精确任务保存在 [prompts.jsonl](../games/escape/assets/prompts.jsonl)。每行含 `model`、`out`、`size`、`quality`、`prompt`；`out` 是 `--out-dir` 下的文件名。背景请求 `1536x1024`，人物请求 `1024x1536`，装置图集请求 `1024x1024`，均使用 `quality: "high"`。

```bash
imagegen generate-batch \
  --input games/escape/assets/prompts.jsonl \
  --out-dir output/imagegen/escape-v2 \
  --concurrency 2 --max-attempts 2 \
  --dry-run
```

本次批量生成采用并发数 2。先确认一张样图的风格和结构，再扩展同系列素材，减少整批返工。`n` 用于同一提示词的多个变体，不代替不同素材的独立任务。

### 使用参考图生成行走动画

本次先生成「4 个角色 × 4 个方向」的身份图集，再把每个角色所在的一行裁成四方向参考图。每个角色通过一次 `edit` 生成完整的 16 格行走图集，避免逐帧独立生成带来服装、脸型和比例漂移。

| 行走图集维度 | 内容 |
| --- | --- |
| 4 行 | 朝下／正面、朝上／背面、朝左、朝右 |
| 4 列 | 左脚向前、过渡落脚、右脚向前、过渡落脚 |

身份图集的「行代表角色、列代表方向」与行走图集不同，裁切和运行时取帧不能混用。参考图和精确编辑提示词见 [walk-prompts.json](../games/escape/assets/walk-prompts.json)。以下示例提取第一个角色的已存提示词，输出新版本，不覆盖原稿。

```bash
mkdir -p tmp/imagegen
"$IMAGEGEN_PYTHON" -B - <<'PY'
import json
from pathlib import Path

jobs = json.loads(Path("games/escape/assets/walk-prompts.json").read_text())
Path("tmp/imagegen/walk-0-prompt.txt").write_text(jobs[0]["prompt"] + "\n")
PY
imagegen edit \
  --model gpt-image-2.5 \
  --image games/escape/assets/source/character-0-reference.png \
  --prompt-file tmp/imagegen/walk-0-prompt.txt \
  --size 1024x1536 --quality high \
  --out output/imagegen/walk-0-v2.png \
  --dry-run
```

提示词明确保留身份、服装、发型、色彩、身体比例、投影和光向；要求完整头脚、透明留白、固定格数，无文字、网格线、地面或外部阴影。整张生成有助于一致性，但不保证格子等距、步态不同或循环流畅，仍需逐格和播放检查。本次在首个角色确认后，再并行编辑其余三个角色。

运行时按方向选行，每 125 ms 切换一列，即每秒约 8 次姿态切换；人物位置插值和画面渲染独立进行。动画来自预生成图集，游戏运行中不调用生图 API。取帧实现见 [canvas.ts](../games/escape/web/canvas.ts)。

## 先约定美术规格，再写提示词

- 固定投影、调色板、光照方向、人物比例、像素笔触密度和墙地分界；本次规格见[密室逃脱美术规格](../games/escape/assets/art-direction.md)。
- 分开生成背景、交互装置和角色。灯光、积水、粒子、设备状态与 UI 由程序按游戏状态叠加，减少背景与交互位置冲突。
- 背景提示词写清中心可行走区域和边缘家具范围。固定镜头，排除消失点和菱形等距构图，不把人物、中央交互装置、文字或 UI 烘焙进背景。
- 图集写清行列含义、完整轮廓和每格留白。参考图通过编辑接口传入，并在每次提示词中重复需要保持的属性。
- 每次只修正一个具体问题，例如步幅、背景残留或角色比例；确认结果后再继续。风格参考站点用于理解视觉方向，展示截图不能直接视为已授权素材。

## 实际踩坑与处理

### 请求尺寸不等于返回尺寸

本次接口曾将请求 `1536×1024` 的背景返回为 `1672×941`，请求 `1024×1024` 的装置图集返回为 `1263×1246`。四张行走图集请求均为 `1024×1536`，实际分别为 `1241×1267`、`1086×1448`、`1214×1295`、`1246×1263`。这是本次服务的观察结果，不应把请求尺寸当成裁切依据。

处理时读取图片的实际宽高，再按 `round(col * width / columns)` 和 `round(row * height / rows)` 计算边界。若视觉格线也不规则，需要人工校正或重新生成，比例裁切不能自动修复布局错误。背景先按目标宽高比居中裁切，再缩放，避免直接拉伸破坏人物和场景比例。

### 有 Alpha 通道，仍可能带白底

第一张工程师行走图集虽然为 RGBA，仍有半透明白色蒙层；仅删除低 Alpha 像素没有去净。白底扩大了内容包围盒，归一化后角色比站立时明显缩小。

本次仅对这张受影响的 `walk-0`，逐格从外边缘做连通区域清理。候选条件为 `a < 8`，或 `min(r,g,b) > 135` 且 `max(r,g,b) - min(r,g,b) < 55`，命中的外连通像素 Alpha 设为 0，保留护目镜等封闭高光。实现见 [prepare.py](../games/escape/assets/prepare.py) 的 `clear_white_matte`。

这些阈值适用于本次素材，不能作为通用抠图参数；浅色衣服可能与背景连通。先在深色、彩色和棋盘背景上检查，再决定是保留原生透明、定向清理还是重新生成。透明像素保留非零 RGB 并不等于有残底，应结合 Alpha 判断。

本次初始角色和装置提示词曾要求洋红底，而行走图集要求原生透明。实际处理脚本主要使用 Alpha，并未实现通用洋红抠图；新素材若返回不透明色底，需要单独验证和处理，不能直接复用假设。

### 图集统一脚底与比例

本次处理脚本以 `Alpha > 48` 获取内容边界，角色单格归一化为 `96×144`，装置为 `192×192`；内容缩放到单格宽高各减 12 的范围内，水平居中，脚底距格底 6 px。背景为 `1536×1024`、WebP 质量 93；透明图集使用无损 WebP。

这些是本游戏的参数。后续素材需要检查头身比例、脚底接触和各帧缩放，不能只依靠格子尺寸一致。已有素材校验为本次人形行走帧加入 `contentHeight / contentWidth > 1.3`，用于发现白底撑大包围盒的回归，不适用于所有道具或角色体型。

## 验收与留档

1. 记录提示词、模型、生成／编辑方式、输入参考图和批准的原稿；原稿与运行时素材分开保存。
2. 检查实际尺寸、Alpha、非空帧、头脚裁切、脚底锚点和各帧比例。在真实游戏中播放四向循环，检查重复帧、抖动、首尾衔接，以及站立与行走的尺寸变化。
3. 在 PC 和手机上检查角色识别、四人重叠、交互区域与背景的关系。静态图集预览不能替代实际动画验收。
4. 归一化并压缩后，记录源文件和产物的 SHA-256、尺寸、体积、Alpha 和帧边界，保证产物与记录一致。本次样例见 [manifest.json](../games/escape/assets/manifest.json)；现有检查可在仓库根目录运行 `node --test games/escape/tests/assets.test.mjs`。
5. 游戏引用应用相对路径，例如 `/escape/v1/walk-0.webp`，由平台资源清单解析到当前提交的 CDN 地址。本次直接拼接旧 `assetsBaseURL` 曾导致新增素材指向缺失路径，接入时需验证真实加载地址。
6. 按[加载与资源发布规范](../rules/loading.md)发布公开素材，保留原稿和提示词供追溯，避免把原稿和客户端载荷重复打入生产二进制或镜像。完成后清理本次临时提示词、缓存和失败产物。

本次游戏验收记录及截图入口见[密室逃脱验收记录](escape-verification.md)。纯文档维护只需检查链接、示例语法与 CLI dry-run，不需要重新生图或发布游戏。
