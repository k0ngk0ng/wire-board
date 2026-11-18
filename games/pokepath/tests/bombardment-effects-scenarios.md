# 轰炸绘图验收

实现前列出失败场景：圆形／旋转矩形瞄准区、中心叉号、炮弹完整图像的旋转缩放、未加载素材的灰色回退、落点投影、雷电逐段显示、命中后 400 ms 脉冲，以及弹道／面积／轰炸三种独立透明度。

隔离原版绘图片段，30 个固定姿态逐 RGBA 比较，不运行原版战斗或 Main。命令 `node --test games/pokepath/tests/bombardment-effects-e2e.test.mjs`；产物位于 `.local/pokepath/verification/bombardment-effects/`。图像加载使用受控测试纹理，真实资源加载由主页面验收覆盖。
