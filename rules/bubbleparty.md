# 泡泡堂开发规范

本游戏同时遵循[通用规范](common.md)、[即时游戏开发规范](realtime.md)、[控制器开发规范](controller.md)。界面、加载、音效等改动按 [AGENTS.md](../AGENTS.md) 的专题索引读取对应规范。

手机方向摇杆固定在左侧，放水泡按钮固定在右侧，横屏与全屏不能套用桌面右侧单列布局。两组触摸区域不得重叠，按住移动时可用另一根手指放泡；抬起放泡手指不能中断移动。布局验收命令为 `node games/bubbleparty/tests/mobile-controls-browser.mjs`，结果和截图写入 `.local/bubbleparty/mobile-controls/`。本次 CSS 与浏览器脚本的命名、目录和禁用注释检查使用 `node games/bubbleparty/build/lint.mjs`，范围在 `build/lint-rules.json` 显式登记。

实际房间的双指输入使用 `python3 -B scripts/bubbleparty/check-mobile-oct23.py --web <本次构建目录> --out .local/bubbleparty/native-mobile`；脚本使用该构建并补齐游戏的公开资源，验证 320、390、844 横屏和桌面原生输入。产物包含截图、输入轨迹、实际可见区域和构建摘要。

## 相关资料

- [模块目录](../games/bubbleparty/)
- [玩法、实现与验收说明](../games/bubbleparty/README.md)
