# 原版物理震屏验收

震屏属于最终显示变换，原版 `VIDEO_UpdateScreen` 将逻辑画面复制到独立显示表面，按剩余次数的奇偶上下移动，并以调色板索引 0 填充露出的边缘。不能修改逻辑地图、人物坐标、碰撞或后续画面备份；场景波动与物理震屏保留原版先后顺序。

## 实现前失败场景

1. 仅递减震屏次数，没有实际画面位移；移动方向与原版奇偶相反，最后一次漏画或持续多画一帧。
2. 用 CSS 变换移动整个页面、工具栏或画布，缩小游戏区域，或者拉伸画面而没有原版边缘填充。
3. 将位移写回逻辑屏幕、地图或人物，重复积累偏移，改变碰撞和物品搜索；文字、人物和背景的显示位移不一致。
4. 原版局部更新错误消耗震屏次数，重新震动整张画面，或未保留局部更新区域之外的前一显示画面。
5. 保存只保留剩余次数，没有保留已显示的奇偶帧、显示图和局部更新历史；新实例恢复后先重画错误方向或重复消费一次。
6. 没有震动、震幅为零、停止后的完整更新或画面渐变仍使用上一次震动；原版淡化时从备份表面复制的画面被误替换。
7. 用新核心读旧核心内存，或将渲染夹具当作真实剧情震屏验收。所有新构建独立保留，主线须从真实新游戏继续。
8. 显示图保留被完全覆盖的历史依赖，长期更新耗尽节点；正常无震屏场景永久进入离屏图而失去已有移动预测。
9. 真实祈雨使用其他核心、调试根档或直接调用震屏接口；只验证绘制图中存在复制节点，却没有核对当前最终显示的位移、调色板索引 0 边缘及手机／PC 实际画布。真实入口须绑定已通过的完整剧情报告；演出中的两个位移方向分别保存并用原核心继续，不能用移植夹具代替真实剧情。

先在真实新游戏到达的客栈场景中进行独立呈现测试。只调用公开原版绘制、震屏和屏幕更新函数，不修改剧情状态或资源。原版 C 显示表面、独立逻辑画面和浏览器渲染逐像素比较；覆盖 PC、手机竖屏和横屏。此项通过后仍须验证真实剧情与存档，不增加主线段数。

## 独立结果

新候选编译固定版本的原版 `VIDEO_UpdateScreen`，保留完整复制、局部更新、奇偶位移、边缘填充及递减顺序。首次震屏时，从当前逻辑画面建立只读绘制图；原版复制到独立显示表面的操作继续由原生执行，并通过已有的复制、填充和缩放节点呈现。完整覆盖时丢弃前一显示图依赖，局部更新保留区域外的显示历史。画布及页面不做 CSS 位移或尺寸调整。

原版 `VIDEO_FadeScreen` 同样保留实际循环、备份屏幕混合和计时，只增加混合及最终显示的观察。用于颜色渐变的源表面明确记录，不能以战斗背景代替普通画面。显示变换不写回逻辑地图、人物或碰撞。

候选均独立保留，没有提升默认核心，也没有接收旧核心内存：

| 用途 | Wasm SHA-256 | 目录 |
| --- | --- | --- |
| 原版数值 | `0230e13219724a65e8a2c3d08560ab53766fa1350cf81b7c43e6055c98ad97d6` | `.local/pal/runtime-candidates/presentation/original/` |
| 仅开发加血 | `c9411008de561adb235b21f0839ed2f6b7fdd60c88818225d6d3e1dd842509aa` | `.local/pal/runtime-candidates/presentation/hp-9999/` |
| 原版像素参考 | `5f8e57f599c375e53aa6c5ecfc7a20c205966af5b48a5617013b0c5bcff2a5ab` | `.local/pal/runtime-candidates/presentation/reference/` |

独立原生测试通过 91 个显示位置、7 次全新实例保存恢复和 6,000 次完整显示覆盖。包含奇偶方向、最后一帧、零次／零幅度、199 行幅度、场景波动与整张画面位移、局部更新不消费次数且保留区域外历史，以及原版备份画面渐变中的震屏和续档。原版 C 参考显示像素与浏览器实际 `SceneRenderer`／`PalStage` 在 PC、手机竖屏和横屏完成 273 次逐像素比较。截图已复核，画布维持视口内最大原比例。

Go 引擎另通过捕获、编码、全新实例恢复、输入清理及 8 次正常场景续行；震屏结束后重新得到普通场景绘制和公开移动状态。原有场景波动的快照／续行回归通过。结果为 `.local/pal/verification/screen-shake-engine.json`。

采用相同呈现适配的完整结局夹具为 `.local/pal/presentation-ending-fixture/rules.wasm`，SHA-256 为 `c73a3f88c3b78e817eca090b345d8d7cbfba71fdf03db295855d5b9b64a685df`。原生三次跳过等待续档、署名页与关闭回归通过；真实 PC／手机 Board 的影片、音轨、3 次私人保存和署名页点击回归也通过，结果位于 `.local/pal/presentation-ending-board-browser/`。夹具仍明确拒绝进入剧情核心目录。

上述独立结果只覆盖呈现夹具。真实剧情与平台存档另按下节验证；完整音画性能和发布仍独立验收，开发加血不进入正式数值。

## 真实地魔兽登场与个人存档

使用已通过的完整剧情报告中的真实入口，继续族长邀请灵儿留任祭司后的原版演出，直到地魔兽登场对白。入口 `prayer-priestess-offer-reading-state-1.json` 的 SHA-256 为 `53d164ac209376da230ea32e98a1e6411c2f0e28d841921066b7b883290bcf2e`；登记在 `tests/browser-campaign-roots.json`，同时绑定真实核心、完整剧情终点、报告摘要及确实保存过的标签。使用隔离开发核心 `2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa`，没有调用震屏接口、改写原始脚本或替换核心标识。

规则侧通过 400 次正常到期回调和 3 次普通阅读，观察到第 232 场景的 ±1、±2、±4、±5 像素位移。8 种状态分别在演出中保存，并由同一真实核心的全新运行器继续；最终显示图、队伍、金钱及逻辑坐标一致，演出正常抵达原版地魔兽对白。保留 28 个真实显示状态及 8 份冻结存档，位于 `.local/pal/verification/prayer-shake-story/`。

真实平台页面另通过 PC 与 320px 手机各 25 帧、合计 50 帧逐像素核对。参考图使用同帧逻辑画面，再独立按行计算原版上下位移和调色板索引 0 边缘；与实际游戏 Canvas 的全部 64000 像素比较，不以绘制节点存在代替可见画面。两端均覆盖 ±1、±2、±4，额外的 -5／+5 分别由 PC／手机采到。另有 PC 2 次、手机 3 次采样未与当前视图匹配，保留在诊断计数中，不计入通过的像素比较；此处不宣称所有显示帧或持续帧率均已验收。

PC 和手机分别通过平台按钮在真实震屏中保存退出、本人继续。保存时的最终显示图及完整公开边界恢复一致，随后正常完成演出。画布分别保持 1280×800、320×200，没有虚拟按键，截图已复核。本地素材入口返回 404，画面通过已发布 CDN 清单加载，未新增上传。

结果、实际画布 PNG、同帧视图和保存恢复边界位于 `.local/pal/prayer-shake-board-browser/`。本项验证真实地魔兽登场的物理震屏与平台个人存档，不替代菜单／战斗性能、全剧情每帧复盘或最终包／镜像检查。

在一个终端启动真实页面夹具：

```sh
. scripts/env.sh
PAL_SHAKE_BOARD_BROWSER=1 go test -v ./internal/server -run '^TestPALGenuineShakeBoardBrowserFixture$' -count=1 -timeout=16m
```

另一个终端先构建只用于观察的像素探针，再运行浏览器检查。结束后停止夹具：

```sh
. scripts/env.sh
node --input-type=module -e 'import {createRequire} from "node:module";const require=createRequire(new URL("./web/package.json",import.meta.url));await require("esbuild").build({entryPoints:["games/pal/tests/story-shake-probe.ts"],bundle:true,format:"esm",outfile:".local/pal/prayer-shake-board-browser/probe.js"});'
python3 -B games/pal/tests/story-shake-board-browser.py
touch .local/pal/prayer-shake-board-browser/stop
```

单独复现真实规则与 8 种冻结状态的继续：

```sh
. scripts/env.sh
PAL_PRAYER_SHAKE=1 go test -v ./internal/server -run '^TestPALGenuinePrayerPhysicalShakeAndSavedContinuation$' -count=1 -timeout=5m
```

## 复现

```sh
. scripts/env.sh
PAL_NATIVE_WASM=.local/pal/runtime-candidates/presentation/reference/rules.wasm PAL_ORIGINAL_PIXELS=1 node games/pal/tests/screen-shake-native.mjs
PAL_NATIVE_WASM=.local/pal/runtime-candidates/presentation/hp-9999/rules.wasm node games/pal/tests/screen-shake-native.mjs
python3 -B games/pal/tests/screen-shake-browser.py
PAL_PRESENTATION_CORE=.local/pal/runtime-candidates/presentation/hp-9999/rules.wasm PAL_WAVE_CORE=.local/pal/runtime-candidates/presentation/hp-9999/rules.wasm go test -v ./games/pal/engine -run '^(TestOriginalScreenShakeProjectionAndFreshSavedField|TestOriginalSceneWaveProjectionAndSavedContinuation)$' -count=1 -timeout=8m
```

构建分别使用 `build/native.py --out <目录>`、增加 `--test-battle-hp 9999` 或 `--reference-font`。完整结局回归使用 `--ending-fixture --ending-credits --out .local/pal/presentation-ending-fixture`，Go 署名页测试增加 `PAL_PRESENTATION_ENDING=1`，Board 测试增加 `PAL_ENDING_BOARD_PRESENTATION=1`，浏览器脚本增加 `--credits --presentation`。原版编译日期会影响实际核心摘要，以每次构建清单为准。
