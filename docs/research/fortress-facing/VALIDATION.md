# 疯狂坦克左右贴图修复验收

根因是 v3 动画元数据把 I16 主图块误标为右向、另一图块误标为左向。主图块实际朝左，另一图块实际朝右。所有车型的动作索引、帧标签和生成器已统一修正；坡面帧的方向符号同步调整，保持左右朝向落在同一地形坡面上。

图集像素、原始锚点、矩形、动作帧数、时序和服务端坐标／弹道均未改变。v2、v3 已发布对象不覆盖、不重新上传。修改仅涉及前端帧选择、素材元数据、生成器与验收工具。

## 结果

- 23 项疯狂坦克前端检查通过，覆盖左右坡面、分组边界、退化支撑和原始锚点。
- 14 张图集、4,144 条原始记录及 8,288 个方向帧均通过来源像素和元数据验证；186 个 v2 媒体文件与 v3 图集的本地发布副本摘要通过。
- 完整应用浏览器验收通过 33 项检查，保留 33 张截图。14 辆坦克 × 左右方向均将游戏画布实际像素与独立确定的原生方向图块比对，最低匹配率超过 96%。
- 使用原生鼠标／键盘／触屏验证 1280px 向左移动与普通弹、320px 向右移动与特殊弹；核对位移符号、服务器 facing、Fire 动画及炮弹路径起点方向。
- 完整应用配置真实 CDN 基地址，测试阻断 CDN 请求后核对同源回退画面。该设置既验证回退，也允许浏览器读取画布像素，避免跨域画布限制。正常 CDN 媒体与回退使用同一不可变图集。

证据保存在 `.local/fortress-facing/`：`frontend.txt`、`source-pixels.txt`、`assets-dist.txt`、`facing-acceptance/report.json`，以及对应的逐像素结果与截图。`declared-directions.png` 保存修复前的错误标签对照；`native-slopes.png` 保存原始坡面帧对照。摘要见 [evidence.json](evidence.json)。

```sh
. scripts/env.sh
npm --prefix web run build
node --experimental-strip-types --test games/fortressblue/tests/*.test.mjs
python3 -B scripts/fortressblue/verify-assets.py --dist
python3 -B scripts/fortressblue/check-refactor.py --only facing --out .local/fortress-facing/new-acceptance
```

`--only facing` 启动隔离数据目录中的完整应用，使用真实创建、选车和操作接口，不注入对局状态。其临时二进制、数据库和浏览器 profile 会在结束后清理。原始像素重验需已有解码依赖，可运行 `.local/board-art-env/bin/python -B scripts/fortressblue/verify-animation-assets.py`。本轮不修改服务端规则，因此不扩大到完整对局矩阵。
