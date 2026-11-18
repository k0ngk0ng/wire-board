# 地图图层与部署预览

地图按当前路线的原始尺寸绘制。拾取第 3-1 或第 9-2 路线的秘密道具，以及第 8-1 路线的钥圈儿机关触发后，客户端按房间状态按需加载该路线的另一张背景。两端使用已有的素材摘要校验；未触发的路线不会加载隐藏背景。

动态地图图层复现原版的正弦明暗变化和逐帧过渡。关闭时不绘制，也不累计动画相位。切换路线时延续同一局的效果状态；重复绘制同一帧不会把渐变加速。

其他玩家切换路线时，客户端先校验并注册该路线的数据，再导入收到的房间状态。相同路线的常规更新继续同步接收；已经过期的下载结果不会把浏览器倒回旧路线。

进入部署模式后，浏览器从当前的 Go/Wasm 状态查询部署位置。服务器接受动作时复用同一资格判断。空位、普通占位、可搭载位置和不适用地形各自着色；指向有效位置时显示射程。鼠标按地图图格命中，手机只在没有直接命中图格时使用扩大的触控范围。同行玩家已经占用的位置不会触发错误部署，也不会取消当前选择。

```sh
. scripts/env.sh
npm --prefix web run build
node games/pokepath/tests/map-surface-oracle.mjs
node games/pokepath/tests/battle-body-oracle.mjs
go test ./games/pokepath/tests -run '^TestNativeDeploymentPreview' -count=1
node games/pokepath/tests/native-platform-browser.mjs --standard-web --maps
node games/pokepath/tests/native-platform-browser.mjs --standard-web
```

地图对照报告记录 47 张原始尺寸画布的完整 RGBA 摘要、三个机关前后的背景及七个动态图层时刻。全部匹配原版，见 `.local/pokepath/verification/map-surface/result.json`。战场同帧对照增至 38 组，覆盖部署预览、道具地形、搭载与非法目标，见 `.local/pokepath/verification/battle-body/result.json`。射程颜色和一般预览已对照，所有特殊能力改变候选点射程的情况仍需补验。

正式双人房间在第 3-1 路线用真实鼠标触发机关，验证了隐藏背景的按需请求、两端换图、切到尚未加载的路线以及返回后仍绘制解锁背景。两端在能区分两张背景的 127 个颜色通道上均无差异，见 `.local/pokepath/native-browser/run-caf4aa72/result.json`。常规双人回归同时验证点击同伴已占用的图格后仍可继续部署，见 `.local/pokepath/native-browser/run-59ff71cb/result.json`。

[候选图格的射程](deployment-range-contract.md)记录地形、装备和共享照明变化对预览的影响。
