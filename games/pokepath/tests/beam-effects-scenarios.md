# 光束与连接线像素对照

先固定失败场景：普通光束颜色／白芯与渐隐、精神光线、加农水炮螺旋、暗影之光波动和粒子、锚链环、信号光束色相、冰冻炮颜色，以及正负电连接线的位移。透明度、方向与结束零透明边界均覆盖。

只提取原版 Beam／LinkBeam 的绘图方法到隔离夹具，不构造原版规则对象、不执行伤害。正式实现独立维护绘图。48 个姿态逐 RGBA 比较；命令 `node --test games/pokepath/tests/beam-effects-e2e.test.mjs`。PNG、来源摘要和结果保存于 `.local/pokepath/verification/beam-effects/`，不代表碰撞规则已经一致。
