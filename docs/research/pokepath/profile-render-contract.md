# 训练家面板显示对照

对照目标为 PokéPath TD v1.7.3 的 `ProfileScene`。测试从研究源码中提取 DOM 绘制、文字更新和头像选中标记，配合原版 CSS 与固定状态绘制参考面板；不加载原版 Main、规则、音频或存档程序。产品侧执行本项目的 React 组件与 `AssetStore` 字体加载器。

```sh
. scripts/env.sh
node games/pokepath/tests/profile-render-oracle.mjs
node --test games/pokepath/tests/frontend-assets.test.mjs games/pokepath/tests/number-format.test.mjs
```

结果保存在 `.local/pokepath/verification/profile-render/result.json`，包含研究源码与产品源码摘要、固定输入、21 项统计文字、面板几何、截图摘要与全部 RGBA 通道比较。26 组对照覆盖 10 种语言，以及四主题与 NGC、PressStart2P、Lexend、Unown 字体的组合。比较范围为完整的 720 × 624 面板，不遮罩差异；资源和字体完全加载后才截图。

显示约定：

- NGC、Lexend 和 PixelMPlus 字体缩放为 120%，Unown 为 135%，其余为 100%。使用平台内已校验的原字体。
- 数字按原版语言分组，保留原始小数文本，不使用浏览器地区格式取整。共享金币达到原版阈值时使用两位小数的科学记数法。
- 资料总量包含 257 个收集编号和 347 个敌人；训练家面板按原版显示 256 个收集完成度及 345 个敌人种类上限，不修改引擎事实总量。
- 成就图像使用已校验 URL 作为按钮背景，保留原版缩放、滤镜与外轮廓。仅在打开该面板时加载。
- 原版参考截图缺少成就或路线图像时，保留原始截图与差异报告，不据此删除正确素材。

该对照证明固定状态下训练家面板的显示，不代表整款游戏的所有面板、战斗动画或多人运行都已达到像素一致；它们各自保留独立验收记录。
