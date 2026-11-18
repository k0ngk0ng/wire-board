# 固定界面颜色对照

先固定可复现的失败点：空编队卡交错暗色、锁定卡统一背景、敌人面板渐变的终点与高度，以及 `Init.js` 声明的中文无衬线回退字体。颜色只比较没有文字、精灵或动画覆盖的原版像素位置，不把动画帧差当作静态样式差。

第二组覆盖原版内容盒尺寸的卡片菱形、47 px 敌人预览外框、文字描边与标题投影、空编队载入按钮的 0.7 亮度、已校验的菜单纹理。所有值先核对 `ui.css` 和 `UI.js`，再检查真实计算样式；像素总差异另行保留，不用这些定点检查宣称整幅一致。

锁定卡的锁图标使用原版 30 px、48 px 行高、0.5 透明度；与星门槛放在同一条底边为 8 px、行高为 14 px 的容器中，避免独立定位造成锁和数字错位。

执行 `. scripts/env.sh`、`node games/pokepath/tests/build-classic-fixture.mjs`，在本地端口提供 `.local/pokepath/classic-fixture` 后，执行 `node games/pokepath/tests/classic-static-ui.mjs http://127.0.0.1:8792/`。原版来源为 `.local/pokepath/reference-visual/stable-initial-chinese.png`，结果与新截图保存在 `.local/pokepath/verification/classic-static/`。浏览器强制静音，结束后关闭并清理专属 profile。
