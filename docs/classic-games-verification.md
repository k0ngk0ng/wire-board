# 经典游戏内容与手机操作验收

本轮覆盖泡泡堂原版地图、角色、坐骑、推箱和个人拾取音效；疯狂坦克选车图片、随机补给、电脑难度和镜头边界；公共等待房间准备状态和手机长按菜单。

## 还原范围

泡泡堂收录已核验 2002 客户端的全部 112 张经典地图，按原始资源编号及年代标注，保留现有小区 10 和三张自定义地图。Village09 不等同于后来的官方小区 10。地图、角色和坐骑的证据、生成方式及 SHA-256 见 [经典地图研究](research/bubble-classic-maps/README.md)、[平台适配](research/bubble-classic-maps/platform-adaptation.md) 和 [坐骑研究](research/bubbleparty/mounts-original-evidence.md)。

新局使用规则版本 3，20 秒选角后单独倒计时 3 秒，全员确认可提前开战。八名常规角色沿用原始初始能力和成长上限；初始移动为 240 或 300 ms/格。原版灰木箱可推，移动 300 ms/格；固定砖不可推。八种骑乘状态、轨道提速、飞碟穿箱和禁拾取由服务端判定。版本 0／2 的进行中存档不升级规则。

疯狂坦克确认了原版直升机运输、落箱及接触拾取路径，恢复原包补给图。发放间隔和概率是明确的平台参数，不宣称恢复了未取得的原版服务器策略。新房间默认普通，简单降低瞄准精度，困难保留原策略，缺少难度字段的旧局保持困难。高抛炮弹继续飞行，镜头在地图顶部停住，炮弹回落后恢复跟随。

[天气调查](research/fortress-weather/README.md) 确认雪、雾、龙卷风和流星实体；本轮没有把这些尚未整合的天气能力伪装成已实现玩法，也没有加入无法生效的天气补给。现有风力继续生效。

## 验收记录

| 范围 | 结果与保留产物 |
| --- | --- |
| 泡泡堂规则、HTTP、存档与接管 | `.local/bubble-classic-maps/merged-go.txt`、`merged-race.txt`、`server-race.txt` |
| 112 地图 × 2/4/6/8 人电脑完整对局 | 448 局自然结束；430 胜负、18 到时和局；694 次中途恢复状态一致。`.local/bubble-mounts/native-fullmatch-report.json` 和原始 `native-fullmatch-go.jsonl` |
| 地图选择和预览 | 112 张实际图片加载、320px、创建和等待房间换图、失败回退。`.local/bubble-classic-maps/preview-browser-verified/report.json` |
| 经典战场与骑乘 | 两张原样地图、300 ms 推箱中间帧和去重、树前阻挡、320px 摇杆、乌龟/飞碟实际图片及速度。`.local/bubble-classic-maps/browser-native-verified/report.json` |
| 角色选择 | 桌面、320px 原生触控、改选清确认、刷新、到期不开托管、属性差异、八种不同回退图。`.local/bubble-characters/browser-complete/report.json` |
| 个人拾取音效 | 四客户端验证本人一次、队友/敌方/观众零次；掉落、重连和静音不补播。`.local/bubble-pickup-audio/browser-characters/report.json` |
| 准备状态 | 两真人两电脑、保存/保存并准备/撤回草稿和窄屏真实开局。`.local/map-ready-consistency/green-complete/report.json` |
| 疯狂坦克规则和难度 | 34 局完整比赛，含三档难度 × 四种人数；规则和 race 通过，报告绑定 Go 源码摘要。`.local/fortress-supply/rule-results.json` |
| 疯狂坦克补给画面 | PC 与 320px 真实运输、落箱、拾取、刷新和三档选择；运输不消耗玩家操作期。`.local/fortress-supply/browser-verified/report.json` |
| 疯狂坦克镜头 | 桌面和 320px，37 帧炮弹离图时镜头 y=0，43 帧回落，以及拖动、滚轮、双指边界。`.local/fortress-camera/browser-final/report.json` |
| 手机长按菜单 | 320/390px 原生长按、移动、取消、多指；菜单已阻止时仍收到持续输入和 ACK；聊天选字保留。`.local/control-context-menu/game-final/report.json` |
| 前端与公共平台 | 两游戏 86 项前端检查、公共前端 123 项；公共服务端按归属选择，不运行其他游戏规则矩阵。`.local/bubble-classic-maps/merged-frontend.txt`、`shared-frontend.txt`、`.local/ci/shared/result-0.json` |
| 素材发布 | 720 个独立 v4 对象，8,719,375 字节，本地及构建回退、OSS/CDN 的摘要/大小/MIME 均核对，禁止覆盖。`.local/classic-game-assets/oss-publish.json` |

电脑矩阵中有五局共九名角色在放泡后未移动即死亡，已核实出生点均有出口；相邻电脑同时放泡可能封住退路，属于当前策略限制。实体 iPhone 尚未实测；浏览器已验证公共菜单拦截及 WebKit 的 `-webkit-touch-callout` 声明，不能将模拟触屏结果表述为实体 iOS 验收。

## 重复执行

在仓库根目录先执行 `. scripts/env.sh`，构建和测试缓存留在工作区。普通规则检查用 `go test -count=1 ./games/bubbleparty/...`；竞态用 `go test -race -count=1 ./games/bubbleparty/...`。疯狂坦克对应使用 `./games/fortressblue/...`。前端分别执行 `npm --prefix web test -- bubbleparty` 和 `npm --prefix web test -- fortressblue`。

地图完整对局可独立执行：

```sh
mkdir -p .local/bubble-mounts
go test ./games/bubbleparty/server -run '^TestNativeFullmatchAllClassicMaps$' -count=1 -json -timeout 20m > .local/bubble-mounts/native-fullmatch-go.jsonl
python3 -B scripts/bubbleparty/summarize-native-fullmatch.py
```

浏览器检查需先 `npm --prefix web run build`；运行期间不要替换 `web/dist`。对应脚本：`scripts/bubbleparty/check-map-previews.py`、`check-characters.py`、`check-pickup-audio.py`、`check-native-battlefield.py`，以及 `scripts/check-waiting-ready.py`、`scripts/fortressblue/check-camera.py`、`scripts/fortressblue/check-refactor.py --only supplies`、`scripts/mobile-controls/check-browser.py --games bubbleparty`。均保留 JSON、截图和输入记录。

素材可离线核对：`python3 -B scripts/publish-classic-game-assets.py --dist`；网络发布必须显式添加 `--publish`，只创建新对象或验证字节一致的已有对象。

## 清理和发布

各验收脚本关闭本次服务进程并删除临时 SQLite、浏览器 profile、socket 和中间图。保留最终 JSON、截图、必要源码证据和有意义的失败对照；不删除原始素材、其他游戏产物或已发布 OSS 对象。

正式发布按同一提交的 main 检查和已验证二进制产物执行，tag 仅打包发布，不重跑规则矩阵或重编译应用。生产使用版本标签更新，核对镜像、二进制、前端、健康与上下架设置；备份加锁保留最近两份可读数据库，历史镜像使用 `update.sh clean` 清理。
