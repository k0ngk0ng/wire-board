# 战场基础画面

敌人朝当前行进拐点转向，身体刚进入画面就可见。生命和护甲按房间语言显示；「仅最终波次」作用于第 100 波的全部敌人。梦幻和蕾冠王仍隐藏具体生命数值。倒下的敌人使用服务器保留的淡出时间，重生期间使用同一状态中的计时。画面读取这些状态，不改变战斗规则。

普通塔保持原版朝向，不添加原版没有的地面阴影或选中描边。翻转道具仍按伙伴能力翻面或旋转。塔身绘制与点击都按同一层序选择对象，灵幽马位于普通塔上方。悬停使用原版蓝色射程；显示全体射程时按能力着色。

火焰之躯、雪隐、静电、扎根和顺风的光环在粒子关闭时仍可见。合成时保留画面外身体伸进画面的光环，以及倒下时不同图层的淡出表现。

```sh
. scripts/env.sh
npm --prefix web run build
node games/pokepath/tests/battle-body-oracle.mjs
node games/pokepath/tests/native-platform-browser.mjs --standard-web
node --test games/pokepath/tests/{frontend-render,frontend-assets,native-architecture,enemy-effects-e2e,attack-effects-e2e,beam-effects-e2e,map-e2e}.test.mjs
```

对照用路线 1-1 的真实地图和图集，以及 `Enemy.draw`、`Sprite.draw`、塔身及射程的原版绘制片段。34 组固定场景的 720×624 画面均无像素差异；重叠塔身命中和输入未被修改的断言也通过。时钟及随机序列固定，测试包含光环开关和画面边缘。报告和原始截图见 `.local/pokepath/verification/battle-body/result.json`。这组对照未覆盖所有路线、动态地图效果、所有特殊技能或完整连续对局。
