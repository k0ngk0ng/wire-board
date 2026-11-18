# v1.7.3 上游缺陷与导入修复

首次研究导入发现下列 4 处源站问题。研究阶段的失败记录保留在本地验收目录；`manifest.json` 的 `sourceDefects` 保留观察与处理方式。正式产品已独立重写，不发布原版 CSS 或程序；下表的三项 CSS 修复仅是研究历史，弱丁鱼派生图标仍用于当前媒体目录。

| 路径 | 原版问题 | 导入处理 |
| --- | --- | --- |
| `src/css/box.css` | `transform: scale(1.05;` 缺右括号，导致严格 CSS 解析失败 | 补齐函数右括号 |
| `src/css/inventory.css` | `top: -12px; /` 中有多余斜线 | 删除多余斜线 |
| `src/css/scenes.css` | `top: -12px; /` 中有多余斜线 | 删除多余斜线 |
| `src/assets/images/pokemon/normal/wishiwashiSolo.png` | 代码引用该图标，但原版素材清单没有此项，源站返回 HTML | 从实际存在的 `wishiwashiSolo-idle.png` 图集无损裁出第一帧 |

弱丁鱼单独形态的原图集为 192 × 320 px；原版敌人定义为 8 帧、8 个方向，因此派生图标裁切矩形为 `(0, 0, 24, 40)`。裁切只恢复缺失的静态图标，不改变战斗动画图集。研究导入器使用 Node 内置 zlib 解码／重新编码 PNG，保留首帧 RGBA 像素；派生图片的 `derivedFrom` 明确区分原图来源与输出摘要。

研究阶段另有 CSS 路径适配，记录在 `sourceAdaptations`，不列为上游缺陷：`styles.css` 中主题变量的心形图标 URL 为 `../assets/images/icons/...`。这些相对 URL 由消费样式表的上下文解析；原站的 `src/css/ui.css` 消费该变量时可得到正确路径。原版样式表及其适配程序不属于当前产品载荷。

当前媒体发布路径另作一项规范化：未使用的重复图 `marshadow-walk (2).png` 发布为 `marshadow-walk-2.png`。字节、SHA-256 和来源 URL 保持不变，原路径保存于 `sourcePath`。这属于公共 CDN 路径适配，不能放宽公共路径校验。当前文件计数、总字节和逐文件摘要以 `manifest.json` 为准，实际产品索引以 `public/pokepath/v1/resources.json` 为准。

已声明但没有可定位实现的 `soundproof` 被动另见[补充规则事实](../../../docs/research/pokepath/precise-rule-facts.md)。独立实现保留捕获版本的实际规则，不因描述补出新的免疫效果。
