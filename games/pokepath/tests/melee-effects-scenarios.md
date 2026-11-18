# 近战特效像素对照

隔离绘图前列出失败场景：三种剑刃配色／双向挥扫、普通与强化暗影爪、鞭击轨迹与收尾淡出、透明度分组、结束边界、规则输入被绘图修改。

测试只从研究副本提取原版绘图方法到仓库内临时夹具，不运行原版 Main；正式实现独立维护几何与颜色。30 个固定姿态逐 RGBA 通道比较，命令为 `node --test games/pokepath/tests/melee-effects-e2e.test.mjs`，结果及 PNG 位于 `.local/pokepath/verification/melee-effects/`。随机粒子与完整战场布局不计入该证据。
