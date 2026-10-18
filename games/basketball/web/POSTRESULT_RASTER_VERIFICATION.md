# 赛后画面渲染验证

本组件只按场景提供的图块、调色板、滚动分段和精灵列表绘制画面，不读取 ROM，也不执行机器指令。尚未接入正式对局显示。

先在 `accept_postresult_raster.py` 记录图块页、调色板、滚动、精灵顺序、颜色转换及资源缺失等失败场景，再编写浏览器验收程序。实现前因缺少渲染模块失败，保留记录为 `.local/postresult-raster-red.log`。

首个独立参考是原版受控整机录像的最终合影：Scene 7、phase 9，本地第 13,500 帧。该参考通过明确声明的比分及战役进度输入进入结局，不是自然完成十四场比赛的证据。

验收将原版该帧的图块布局、调色板和精灵列表作为输入，图案来自已验证的独立素材导出；在真实 Chromium 中加载索引图集并用 Canvas 显示，与原版截图逐像素比较。61,440 个像素全部一致，缺少 CHR 图块页时拒绝绘制。截图 `.local/postresult-raster/browser.png` 已查看，结果保存在同目录 `result.json`。

```sh
source scripts/env.sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/accept_postresult_raster.py
```

新增行片段路径沿用同一个像素合成器，输入为原图块的页、行、列和调色板引用；离线来源解析器负责证明预取与滚动时序，浏览器不读取 CPU、PPU 事件或原截图来决定绘制。先扩展浏览器验收并保存缺少导出的失败记录，再实现行片段输入。

浏览器逐像素对照已覆盖 8 个主样本和 8 个独立复核样本，共 983,040 个像素，全部一致。包含奖励、舞台、海滨、飞机、字幕与合影；没有剔除精灵或边界像素。每个样本均验证缺失图块页会被拒绝。运行命令追加 `--rows`，结果与截图分别保存在 `.local/postresult-raster/rows-result.json` 和 `rows-browser.png`，日志为 `.local/postresult-raster-rows-green.log`。索引图集已包含全部 8 个球场来源页，共 67 页。

这些结果证明源帧输入下的独立绘制。动态源帧生成、淡入淡出、连续视频调度、服务端快照及真实游戏生命周期仍需后续接入验收，不能据此宣称完整结局播放已完成。
