# 电荷连接在战场中的绘制

先固定失败边界：战场仍传入连接类默认的 8 px 宽度、只绘制帧末的一次连接、把首次连接画在所有塔身之上，以及忽略全局与光束分组透明度。

正常原版路径使用 12 px 连接。部署列表第一座塔在自己的普通弹丸和光束之后绘制连接，后更新的塔身可以遮住这次绘制；帧末再画一次连接。测试使用原版 `LinkBeam.draw` 和完整的独立 `paintBattle`，用固定色块代替塔身来测量遮挡顺序。三个方向、三个透明度共九组未遮罩 RGBA 比较；色块不证明具体宝可梦素材已经一致。

```sh
. scripts/env.sh
node --test games/pokepath/tests/link-runtime-render-e2e.test.mjs
```

结果与两端完整 PNG 保存在 `.local/pokepath/verification/link-runtime-render/`。绘图不修改规则输入；原版提取代码仅在本地测试目录中运行。
