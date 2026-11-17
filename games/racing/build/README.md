# 原版内容扩充

新增宝马 M5、捷豹 XKR、奔驰 CLK GTR，以及德国乡间公路的日夜场景。车辆性能取自各车原始 base CARP；模型保留外观、四轮与原驾驶舱，声音保留原发动机循环。输入及派生文件摘要登记在 `../assets/source/nfs4/content-inputs.json` 和 `content-previews.json`。

离线适配器 `prepare_content.py` 通过现有 GPL-3.0-or-later 转换工具及 speedtools 读取原始资源，适配器按 GPL-3.0-or-later 提供。浏览器只发布转换产物，不打包原始解析器；工具许可不改变 EA 美术、声音等输入资源的权利归属。依赖及原有坐标契约见 [原版资源记录](../assets/source/nfs4/README.md)。

## 复现

以下路径均位于仓库内。原始输入保存在 `.local/racing-content/source`，合并原有输入的目录为 `.local/racing-content/game-root`。取得记录校验每个文件的长度及 SHA-256；未下载完整 ISO，不宣称核验整盘摘要。

使用安装 speedtools 0.38.0 和 Pillow 13.0.0 的 Python 环境，先导出模型、音频及菜单迷你赛道：

```sh
TMPDIR=$PWD/.local/racing-content/tmp PYTHONDONTWRITEBYTECODE=1 \
.local/racing-nfs-reference/asset-venv/bin/python -B games/racing/build/prepare_content.py \
  --source .local/racing-content/source \
  --acquisition .local/racing-content/acquisition.json \
  --game-root .local/racing-content/game-root \
  --converted .local/racing-content/rebuilt \
  --audio .local/racing-content/rebuilt-audio \
  --miniatures .local/racing-content/rebuilt-miniatures --export-only
node games/racing/build/render-previews.mjs \
  .local/racing-content/rebuilt-miniatures .local/racing-content/rebuilt-previews
```

渲染器使用 Three.js 0.180、正交相机和 Chromium SwiftShader，输出 384×216 PNG 与来源报告。车辆缩略图直接解码原 QFS 的 `shp1`，保留 381×178 原像素。两类缩略图无损编码 WebP，逐像素回读检查。

再合并公开产物与服务端／前端数值表：

```sh
TMPDIR=$PWD/.local/racing-content/tmp PYTHONDONTWRITEBYTECODE=1 \
.local/racing-nfs-reference/asset-venv/bin/python -B games/racing/build/prepare_content.py \
  --source .local/racing-content/source \
  --acquisition .local/racing-content/acquisition.json \
  --converted .local/racing-content/rebuilt \
  --audio .local/racing-content/rebuilt-audio \
  --track-previews .local/racing-content/rebuilt-previews
node games/racing/build/lint.mjs --self-test
node --experimental-strip-types --test games/racing/tests/content-assets.test.mjs
```

已存在的不可变模型、音频和几何若字节变化，合并立即失败。新增德国赛道的规则、地形、可破坏物件与场景绑定同一几何摘要。原五车默认编组保留，新增车可在房间选择。

## 验收

先行失败场景见 [内容扩充失败清单](../tests/content-expansion-failures.md)。`content_catalog_e2e_test.go` 覆盖新车完整比赛、德国赛道全部人数／圈数组合及新旧车混跑恢复。真实页面使用：

```sh
python3 -B games/racing/build/check_content.py \
  --base http://127.0.0.1:PORT --dist .local/racing-release/web/dist \
  --out .local/racing-content/evidence/browser
```

`--base` 必须指向自行启动的 `TestRacingBrowserFixture` 本地夹具。检查器保存可重复运行的 JSON 结果、原生输入记录及桌面／320px 截图；浏览器仿真不代表实体手机验收。
