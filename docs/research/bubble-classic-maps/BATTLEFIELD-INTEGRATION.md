# 战场接入与验收诊断

`canvas.ts` 已接入经典地形、v3 原版人物、坐骑和权威推箱运动。`Board.tsx` 按地图 ID 和素材前缀加载当前地图使用的原生资源；切图／卸载中止旧请求，不因普通快照重新下载。人物素材按实际角色和队伍预加载，旧 v2 存档继续使用原 avatar。

原生障碍以 `nativeObjects` 为准；已销毁或移动的对象不会从初始矩阵补回。推箱使用服务端 `boxMotions` 的起止时间和坐标插值，目标静态箱不重复绘制。目标 cell 不再为 4 时立即停止绘制该运动，避免爆炸后残影。减少动态效果仍保留箱体位置插值，使可见箱体与碰撞时间轴一致；不重播过期运动。

人物在普通地形保留本地移动预测。可推箱 4 与普通箱 2 都阻挡，只有两种飞碟允许穿箱，墙和水泡仍阻挡。传送带、方向通道及活动推箱附近使用权威位置平滑，不擅自预测推箱、通道许可或传送带运动。矿车等速度直接使用服务端 `speedMs`；坐骑／人物状态变化清除旧预测历史。该选择优先保证复杂地形位置正确，特殊地形附近输入反馈会更依赖快照延迟。

## Canvas 诊断

在 `[data-bubbleparty-canvas]` 上读取以下属性。绘制计数由对应绘制函数的实际成功返回值生成，不能用它替代最终像素截图验收。

| 属性 | 用途 |
| --- | --- |
| `data-terrain` | `classic` 或 `legacy` |
| `data-native-sprites-requested` | 当前地图实际请求的原始图集数 |
| `data-native-sprites-loaded` | 图集加载成功数 |
| `data-native-sprites-failed` | CDN 与本地均失败数 |
| `data-native-objects-drawn` | 本帧使用原始图集成功绘制的对象数，包含移动箱 |
| `data-native-objects-fallback` | 本帧使用可见回退绘制的原生对象数 |
| `data-players-native` / `data-players-fallback` | 本帧成功绘制的人物／骑乘与回退数；已结束死亡动画的标记不计入 |
| `data-mounted-native` / `data-mounted-fallback` | 骑乘图层成功绘制／回退数 |
| `data-mount-items-native` | 地面坐骑道具原图成功绘制数 |
| `data-box-motions-drawn` | JSON 数组，含本帧 `x/y`、权威目标 `toX/toY` 和 `objectId` |
| `data-prediction-mode` | 本人 `predicted`、`terrain-authority` 或 `authority` |
| `data-render-x/y` / `data-server-x/y` | 本人实际呈现位置与快照位置 |

## 已完成验证

```sh
node --experimental-strip-types --test games/bubbleparty/tests/prediction.test.mjs games/bubbleparty/tests/classic-battlefield.test.mjs games/bubbleparty/tests/classic-terrain.test.mjs games/bubbleparty/tests/characters.test.mjs games/bubbleparty/tests/avatar-art.test.mjs games/bubbleparty/tests/classic-map-options.test.mjs
./web/node_modules/.bin/tsc --project web/tsconfig.json --noEmit
```

34 项针对性测试通过，TypeScript 检查通过。Canvas 测试断言真实 `drawImage` 调用的资源、裁切和坐标，包括灰箱运动中点只绘制一次、v3 蓝色人物、旧 avatar、骑乘／掉骑及失败回退；这些是隔离绘制验证，不是浏览器实机验收。

保存结果：`.local/bubble-classic-maps/source-evidence/battlefield-green.txt`、`battlefield-verification.json`。

## 待主任务浏览器 E2E

- 切换两张原生地图，核对加载计数、实际 PNG 请求及截图，确认没有旧地图覆盖。
- 检查 Village00 居中边界和高树、多格房屋的锚点及遮挡。
- 原生鼠标／触屏推动木箱，逐帧核对 `data-box-motions-drawn` 与箱体像素，不出现目标重复箱；爆炸后没有残影。
- 两队选不同人物，拾取各坐骑并受击掉骑，确认团队色、图层、脚底位置和道具图片。
- 传送带／方向通道／矿轨与飞碟路径检查呈现位置；被困、失焦、重连后没有旧预测残留。
- 阻断 CDN 和本地素材，确认地形、人物、坐骑都有可辨认回退；人物和载具没有身份数字角标。

本次未生成 `dist`，未创建提交、tag 或发布。
