# 方向攻击绘图验收

实现前固定失败场景：冰冻之风与魔法闪耀配色、立方缓出半径、72% 后淡出、面积透明度、粒子开关／分布／旋转、终止边界，以及绘图修改规则输入。

原版绘图和粒子创建代码仅进入隔离研究夹具，正式绘图独立实现。受控随机序列验证粒子分布参数，12 个姿态逐 RGBA 比较。执行 `node --test games/pokepath/tests/directional-effects-e2e.test.mjs`，证据保留于 `.local/pokepath/verification/directional-effects/`，浏览器强制静音。
