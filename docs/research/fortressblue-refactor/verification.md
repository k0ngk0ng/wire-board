# 疯狂坦克重构验收

本轮按官方 Steam 画面、公开网页客户端资源及 REA 对客户端 WASM 的分析重构操作、画面和结算。原版网页运行停在登录页，没有完成原版账号内对局；本文不会把客户端静态分析称为实机试玩。已知证据与未知参数分别见 [reference.md](reference.md)、[native-findings.md](native-findings.md)。

## 已完成的行为

- 按住蓄力、松手或满力单次发射；键盘、鼠标及触屏共用取消和去重规则，去除默认落点预览。
- 近景战场、蓝色仪表台、原生动作／方向／锚点／坡度帧；保持公共房间、玩家生命栏和结算浮层。此前重绘的封面未改。
- 服务端统一管理准备、出膛、飞行、命中、逐车坠落和交接回合；命中前不发布该炮伤害和弹坑。刷新续播，已完成动作不重播。
- 新局双弹无限，校正八种道具，回血弹在命中时治疗双方，火／毒保留在地面。出膛前死亡取消待发炮弹，已出膛炮弹继续。
- 新局选择可移动、受支撑且具有有效射界的出生点；保留旧对局位置。原先车身嵌入地形的席位的独立复验见 [review.md](review.md)。
- 疯狂坦克显式启用 50 ms 系统事件检查。普通模块仍用一秒系统时钟，玩家超时、电脑调度和实时控制帧语义保持原约定。

## 浏览器与构建证据

统一最终运行是 [refactor-final-r2/report.json](../../../.local/fortressblue/refactor-final-r2/report.json)：**41 项通过，22 张截图、7 段录像，JavaScript 错误为 0**。启动与结束核对 33 份实现、夹具及脚本的 SHA-256，以及构建入口摘要，均未漂移。初始化夹具仅提供可重复的开局；之后使用真实 HTTP、SQLite、服务器时钟和原生浏览器输入，不篡改进行中状态或快进时钟。

覆盖短／长／满力、失焦及手势取消、地图拖动／指针锚点缩放／双指缩放／小地图拖动；图片延迟及失败的实际像素；长飞行重连、命中扣血和逐帧落地、回血、毒火到期及支撑被炸毁；致命取消和非致命受击时实际 Fire 第 5 帧；混合电脑、全托管和手动接管；零生命、禁用文案对比度、关闭结算不重开。

根任务另行打开检查了以下最终截图：

- [桌面命中后局面](../../../.local/fortressblue/refactor-final-r2/after-timeline.png)：地形、坦克坡度、公共生命和禁用按钮清晰可见。
- [320px 操作布局](../../../.local/fortressblue/refactor-final-r2/initial-reveal-320.png)：战场、仪表及开火键同时可见。
- [320px 八人战场](../../../.local/fortressblue/refactor-final-r2/eight-seats-320.png)：原生坦克可辨识，公共生命的实际矩形和尺寸记录在对应 JSON。

前端针对性检查 **23 项通过**；TypeScript、前端构建、Go 应用编译及 `go vet` 通过。素材验证通过 **186 份 v2 媒体、14 张 v3 图集、8,288 个朝向帧**。24 个游戏的公共依赖边界检查通过，没有运行其他游戏的规则或完整对局。检查摘要与日志哈希见 [targeted-checks.json](../../../.local/fortressblue-refactor/targeted-checks.json)。

公共平台三组独立检查共 **169 个测试及子用例通过（含 race）**，见 [shared-final/manifest.json](../../../.local/fortressblue-refactor/shared-final/manifest.json)。其中精确系统时钟 E2E 先复现旧的一秒间隔，再验证显式启用后的短节点交付；既有普通模块节奏回归也通过。

## 规则与出生矩阵

普通 Go 全套和最终 race 全套各 **1,064 个测试及子用例通过**，HTTP 恢复 race 另重复 5 次通过。原始 JSONL、检查范围与源码摘要见 [第三阶段验收](backend-stage3-verification.md)及 [backend-stage3/summary.json](../../../.local/fortressblue-refactor/backend-stage3/summary.json)。新出生 E2E 通过 **17 图 × 2／4／6／8 人 × 14 车型 = 952 场、4,760 席位**，检查实际车身净空、支撑、移动和有效射界，并防止全部落位后队友挡住唯一射界。该矩阵不是全部混合车型排列的穷举。

重启 HTTP 测试曾发现旧的「快照必须完全静止」断言不适用于正确运行的短系统时钟；修订只允许原定出膛节点在重启期间恰好到期，不放宽生命、位置、路径、回合和期限约束。

早期独立射击枚举的范围已更正：修复前只枚举 0～40°，不能据此宣称全部合法角度必然自伤。当前实现已另按车型完整 0～90°范围执行 18,382 发枚举；两个不同范围不作直接对比。纠正记录见 [scope-correction.json](../../../.local/fortressblue-refactor/review/scope-correction.json)。

## 复跑

在仓库根目录执行，所有缓存和临时目录留在工作区内：

```sh
. scripts/env.sh
npm --prefix web test -- fortressblue
npm --prefix web run build
python3 -B scripts/fortressblue/verify-assets.py --dist
python3 -B scripts/fortressblue/check-refactor.py
go test -count=1 ./games/fortressblue/...
go test -race -count=1 ./games/fortressblue/...
python3 -B scripts/ci/scope.py run --game shared
python3 -B scripts/check_game_boundaries.py
```

浏览器脚本每次使用新目录，完成后保留报告、截图、录像与采样记录，关闭服务并清理浏览器 profile、测试数据库和临时编译目录。

## 限制与交付范围

原版 Delay 排序、精确弹道／伤害常数及原版更新频率未获确认。当前固定交错回合、物理常数、30 Hz 蓄力换算与地面效果数值仍是明确记录的项目规则，不能称为完全一致复刻。详见 [CONTRACT.md](../../../games/fortressblue/CONTRACT.md)。

最终工作树来源见 [source-manifest.json](../../../.local/fortressblue-refactor/source-manifest.json)。本轮完成本地代码与验收，未创建新版本 tag、GitHub Release 或更新生产。清理只涉及本次临时编译、基线浏览器 profile／测试库、废弃分析中间件和被最终验收取代的运行；保留原版包、正确 REA 证据、必要截图／录像／报告、WABT 重跑工具、已发布媒体及共享缓存。根任务清单见 [cleanup.json](../../../.local/fortressblue-refactor/cleanup.json)。浏览器另外清理 11 个被取代的运行，保留最终目录的 `red-proof.json`、`verification-summary.json` 与 `artifacts.sha256`。
