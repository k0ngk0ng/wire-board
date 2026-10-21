# 原生战场浏览器验收结果

验收脚本：`scripts/bubbleparty/check-native-battlefield.py`。场景清单先写于 `NATIVE-BROWSER-SCENARIOS.md`。

```sh
python3 -B scripts/bubbleparty/check-native-battlefield.py --out .local/bubble-classic-maps/browser-native-new-run
```

最终结果：`.local/bubble-classic-maps/browser-native-verified/report.json`，**10 项检查全部通过**。保存 12 张截图和逐帧输入／物理／Canvas 诊断；报告记录 19 个结果文件的字节数和 SHA-256，逐一复核一致。

本次页面构建 `web/dist/index.html` SHA-256：`227395be8d9cda0234ea347712687b283b96158f71e28d49877151eb7fb92936`。脚本不构建或修改页面。

## 实测结果

| 场景 | 结果与证据 |
| --- | --- |
| 经典小区 00、经典工厂 07 | 使用未经改写的生产 Start；真实选人与倒数后，原生图集全部加载，原生障碍实际绘制数超过 20、回退数为 0，HUD 显示中文地图名。`original-map-1/2.json` 与对应 PNG。 |
| 推箱动画 | 原生键盘正面推动灰箱。记录源格清空、目标 object ID 66、服务端运动持续 300 ms；Canvas 在 4.1～4.9 格之间有实际中间位置。每帧原图绘制数等于权威对象总数，没有额外目标箱。`push-intermediate.json`、`push-midpoint.png`。 |
| 不能推穿树 | 连续原生输入把箱子推到树前，树的 CellWall 与 object ID 91 不变，箱子最远到 x=7，人物不超过 x=6.22。`push-until-wall.json`、`push-wall.png`。 |
| 刷新保存推箱结果 | 动态箱子仍在 x=7，过期运动不重新出现。 |
| 320px 摇杆 | 原生触摸拖动向左，人物实际移动超过半格；松手后输入为 `none`，全程未选中文字。`mobile-joystick.json`、`push-mobile-320.png`。 |
| 乌龟 | 合法走到道具后拾取；实际地面图标、坐骑图层和角色图层成功绘制，公共玩家栏显示骑乘乌龟。服务端 600 ms／格，按坐标与时钟实测 **1.6667 格／秒**。`turtle-ride.json`、`turtle-item.png`、`turtle-mounted.png`。 |
| 飞碟 | 合法拾取后骑乘图层成功绘制；服务端 120 ms／格，实测 **8.3333 格／秒**。穿过灰箱时不产生推箱事件，箱子原位不变；遇树停在 x≤7.22。`airship-ride.json` 和对应道具／骑乘 PNG。 |
| 飞碟重连显示 | 320px 下刷新，服务端仍为飞碟，实际骑乘原图恢复绘制。`airship-mobile-320.png`。 |

已目视检查原样小区／工厂、推箱中间帧、乌龟与飞碟桌面图、320px 摇杆和飞碟图。图片来源、角色红蓝色、箱体位置和公共资源与诊断相符。

## 隔离范围

前两张地图完全使用生产起始状态。后面三张对局通过脚本生成的测试专用 Go overlay 包装 **Start 初始返回值**：局部区域改成测试走廊，放置一只可推箱、一棵树及一个指定坐骑道具。随后全部使用生产 HTTP、SQLite、Apply、OnTimer、View 和浏览器原生键盘／触摸。

没有运行中状态注入、模拟服务器胜负、篡改页面状态或伪造素材。固定场景不证明随机掉落概率，也不代替全部 112 张地图完整对局验收。此报告不宣称所有坐骑或复杂地形组合均已做浏览器测试。

首次跑通后增加推箱时长及绘制去重断言；第二次仅因快飞碟测速窗口过短、有效服务端样本不足失败。最终把飞碟测量区间延长至树前，仍要求至少 3 个不同服务端时刻及同一速度精度，全部通过。未因该问题修改游戏实现。

## 清理

每次结束均关闭所属浏览器和 Go 测试服务，删除该次 profile、SQLite 数据库、临时 Go overlay、临时 socket 目录及服务控制文件。完成后仅保留最终通过的 `browser-native-verified`；删除本次初跑重复产物和采样窗口失败产物。原始客户端、源码、发布素材及其他任务目录均未删除。
