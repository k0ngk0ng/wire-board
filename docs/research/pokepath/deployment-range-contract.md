# 候选图格的射程

部署前，浏览器用同一份 Go/Wasm 引擎试算鼠标或控制器指向的合法位置。试算在副本中按实际部署规则执行，再取该位置的中心、范围形状、外半径、环形内半径和条带半宽。查询不落子，也不改变房间状态。已部署的伙伴移动到其他地形时，预览使用新位置的属性。

已核对高地「无防守」的十倍射程，以及 12 条沙漠路线的沙漠双射／四射射程翻倍。环形保留内圈，星形按原版的直向与交叉条带绘制。宽镜的 X 形条带保留原版显示宽度。

```sh
. scripts/env.sh
npm --prefix web run build
go test ./games/pokepath/tests -run '^TestNativeDeployment(Range|Preview)' -count=1
node --test games/pokepath/tests/native-runtime-e2e.test.mjs
node games/pokepath/tests/battle-body-oracle.mjs
node games/pokepath/tests/native-platform-browser.mjs --standard-web --range
```

引擎测试逐项比较预览和实际部署，覆盖地形、照明、高地十倍、沙漠翻倍及不同形状。战场报告新增 7 组候选射程画面，45 组完整画面均与原版无像素差异，见 `.local/pokepath/verification/battle-body/result.json`。双人正式房间测试在候选图格距中心 135 像素处取样，超出伙伴本身的 125 像素射程。玩家 A 收回照明伙伴后，该点恢复地图底色；玩家 B 不移动指针即可看到范围变化，并在该图格完成部署。报告见 `.local/pokepath/native-browser/run-e1500b2e/result.json`。

这些对照未覆盖全部特殊技能和道具组合，也未测量密集战斗时连续查询的延迟。
