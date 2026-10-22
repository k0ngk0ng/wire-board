# 地图分类与预览核查

2026-10-23。范围为地图选择、预览和玩家可见文案，不改地图规则、开局布局或存档 ID。

## 名称与来源

此前「现有场地」实际包含四张地图和一个随机入口。小区 10 的布局来自官网配图转录，另三张地图使用本项目的生成地形；它们不是留存客户端目录中同名地图的别名。

| 地图 ID | 玩家显示名称 | 主题 | 预览依据 |
| --- | --- | --- | --- |
| `community10` | 经典小区10 | 小区 | 当前 `districtCells`／`districtTiles`，官方截图对应的中央道路、左右建筑布局 |
| `forest` | 森林公园 | 森林 | 当前生成地形的固定样例，箱子分布每局随机 |
| `desert` | 沙丘营地 | 沙漠 | 当前生成地形的固定样例，箱子分布每局随机 |
| `waterpark` | 水乐园 | 海洋 | 当前生成地形的固定样例，箱子分布每局随机 |
| `random` | 随机地图 | 独立入口 | 开局前不展示已选定地图 |

小区 10 没有重定向到 `classic2002-village09`。官网配图左上角为 10，网址文件名 `map11.jpg` 不决定地图编号；既有核查见 [原版参考核查](reference-findings.md)。其他 112 张地图继续使用原始资源 ID、编号与各自预览，九个主题按名称分组。归类是用户界面组织方式，不宣称不同版本编号等价。

## 渲染与证据

原有四图预览是低饱和 SVG 示意图，使用独立颜色和简化房屋轮廓，与实际开局不一致。现在调用游戏已有 `drawPreview`，复用对局的 `ground`／`obstacle` 与 `Town*.png` 精灵；额外传递 `tiles`，避免把蓝房、黄房和双色砖都降级成红房、木箱。示例地形仍取自 `current-map-previews.json`，其来源为生产 `Module.Start/View` 的固定 RNG 样例。随机箱子图仅在预览下保留必要提示「木箱位置每局随机」。

素材来源保持不变，未生成新图片、未覆盖已发布 OSS 对象。小区精灵的第三方来源及审计见 [原版参考核查](reference-findings.md)，REA 静态分析 Evidence 为 `ev_9bd3d2e2ef37e2c94d94ee337e6258f62e70982ff13cfee021f26a5773bc24a6`；该证据不代表官方运行观察。本次没有运行原版客户端，也未新增原版行为结论。

「2002 留存版」「按原资源重建」「编号不对应现行国服」等来源说明从地图分组、预览和玩法速查移除；技术来源与版本边界保留在研究文档、资源清单和源码中。

## 验收

失败场景先记录在 `games/bubbleparty/tests/MAP_PREVIEW_SCENARIOS.md`。分类测试先因缺少新分组失败，渲染测试先因蓝房没有正确绘制失败；实现后 4 项针对测试通过。完整前端构建通过。

浏览器验收使用实际应用、HTTP、SQLite、地图素材与真实下拉选择，不注入游戏状态。命令：

```sh
. scripts/env.sh
npm --prefix web run build
node --experimental-strip-types --test games/bubbleparty/tests/classic-map-options.test.mjs games/bubbleparty/tests/map-preview-render.test.mjs
python3 -B scripts/bubbleparty/check-map-previews.py --out .local/bubbleparty/map-previews-new-run
```

本次产物：`.local/bubbleparty/map-previews-oct23-pass/`。`report.json` 保存 12 项检查，`all-previews.json` 保存全部 112 张原图的 ID、资源地址和解码尺寸；13 张截图覆盖四张现有场地的 PC／320px，以及经典地图的创建、失败反馈、恢复和等待房间。检查 117 个唯一选项、9 个主题归属、预览尺寸、色彩像素、窄屏无横向溢出、创建保存及等待房间换图。

本次清理仅覆盖失败的初次预览验收目录、失败日志，以及验收脚本创建的浏览器 profile、socket、fixture 数据和 URL／停止文件。保留最终报告、截图、112 图清单、通过的测试记录和服务器日志；不清理其他任务的构建或缓存。
