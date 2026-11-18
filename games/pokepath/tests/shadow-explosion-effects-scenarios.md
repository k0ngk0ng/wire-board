# 黑影爆裂绘图验收

实现前固定检查：原位置下移 8 px、立方缓出、椭圆比例、八条裂纹、450 ms 渐隐及不修改传入事件。使用原版独立绘图方法与自写方法对照六个姿态，不运行原版规则；执行 `node --test games/pokepath/tests/shadow-explosion-effects-e2e.test.mjs`，结果与 PNG 留在 `.local/pokepath/verification/shadow-explosion-effects/`。
