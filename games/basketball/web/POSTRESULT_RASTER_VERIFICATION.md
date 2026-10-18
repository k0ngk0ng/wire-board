# 赛后画面渲染验证

本组件只按场景提供的图块、调色板、滚动分段和精灵列表绘制画面，不读取 ROM，也不执行机器指令。尚未接入正式对局显示。

先在 `accept_postresult_raster.py` 记录图块页、调色板、滚动、精灵顺序、颜色转换及资源缺失等失败场景，再编写浏览器验收程序。实现前因缺少渲染模块失败，保留记录为 `.local/postresult-raster-red.log`。

首个独立参考是原版受控整机录像的最终合影：Scene 7、phase 9，本地第 13,500 帧。该参考通过明确声明的比分及战役进度输入进入结局，不是自然完成十四场比赛的证据。

验收将原版该帧的图块布局、调色板和精灵列表作为输入，图案来自已验证的独立素材导出；在真实 Chromium 中加载索引图集并用 Canvas 显示，与原版截图逐像素比较。61,440 个像素全部一致，缺少 CHR 图块页时拒绝绘制。截图 `.local/postresult-raster/browser.png` 已查看，结果保存在同目录 `result.json`。

```sh
source scripts/env.sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/accept_postresult_raster.py
```

这项结果仅证明最终合影的独立绘制。舞台、海滨、飞机及字幕场景的分段取图、横向滚动、精灵遮挡、淡入淡出、服务端快照及真实游戏生命周期仍需后续验收，不能据此宣称完整结局画面已完成。
