# 普通弹道验收

- 呱呱泡蛙发射宽 32.5 px 的弹道时，按弹道图集的实际帧尺寸确定中心。不能把塔的中心当作弹道中心，也不能把半像素取整。
- 普通弹道抵达敌人判定圈的边缘时继续飞行。下一次移动进入判定圈后，敌人只结算一次直接伤害。高速度弹道经过判定圈时不能穿透目标。
- 弹射从被击中的敌人附近创建新弹道。用敌人中心确定下一目标，再以新弹道的帧尺寸和至多 3 px 的发射偏移确定起点。上一次飞行的用时不能缩短新弹道的寿命。
- 对照路线 1-1 首波中三只初始伙伴的实际发射原点。保留发射、命中、弹射和整波结算的原版与独立实现记录。
- 分别让三种普通弹道斜向飞行、反向飞行和以降低的特效透明度显示。弹道贴图保持原版朝向，在小数像素位置和不同动画帧逐通道比较 Canvas。

```sh
. scripts/env.sh
go test ./games/pokepath/engine -run '^TestCoreOrdinaryProjectile' -count=1 -json
node games/pokepath/tests/first-wave-reference.mjs --settlement
node --test games/pokepath/tests/projectile-flight-render.test.mjs
```

原版路线 1-1 的图格 1 上，小火龙与木守宫的首发中心为 `(250, 130)`，呱呱泡蛙为 `(250.25, 130)`；独立规则得到相同坐标。三种贴图各比较 3 帧，9 帧的 RGBA 通道均无差异。截图和摘要见 `.local/pokepath/verification/projectile-flight-render/result.json`。

普通弹道按相邻碰撞采样点之间的距离查找第一个进入判定圈的位置。弹射重新分配弹道标识，保留剩余次数与伤害递减记录，并重新计算贴图中心和飞行时间。首波四组结算保持一致；完整命中顺序仍见[首波连续对照](first-wave-reference-scenarios.md)。
