# 征服时代随机开局与行动播放验收

本轮保留原内部计算，增加随机出生、逐步行动播放和清楚的行军次数显示。此前拟统一每条计划消耗一次的方案已撤回，未发布到线上。

## 最终行为

- 内部仍以 3.2 行动点开局，征兵／行军各消耗 1 点，筑堡 0.5 点、瞭望塔 0.3 点、城镇 5 点、解散 0.1 点，调税不消耗行动点。剩余点数按原精度半数结转，收入、人口和战斗公式保持原样。
- 右上角的原始点数带「点」单位。计划条数独立显示，按钮下方显示「可行军 N 次」，按剩余点数计算完整行军次数，仅用于展示，不回写服务端数值。
- 新局从全部地图地区中随机选择出生点，玩家之间不得陆地或海路相邻。使用私有种子，结果写入存档；恢复既有对局不重新随机，也不改变已接受的计划、点数和期限。
- 可见行动逐条执行。行军约 1 秒，征兵、建设等约 0.9 秒，到期才提交资源和地图结果，再停留至少 0.5 秒显示结果。最后一步完成结果停留后才弹出胜负。无效计划明确标为未执行。
- 行动提示显示玩家、地点、路线、兵力与结果。镜头显示当前行动区域，回合结束后恢复先前视野；减少动态效果时保留文字与静态标记。刷新不重播历史动作。
- 军队使用不可见的较大命中区，不增加可见矩形或编号；相邻标记避免命中区重叠。零兵力君主与公开的王冠位置仍可见、可选择。
- 随机开局暴露的美国电脑僵持已修复：策略可根据本席已看到的战报估计补充损失的兵力，并受当前可见人口限制；不读敌方私有计划或金币，不改实际规则和结果。

截图：[手机行动播放](screenshots/conquest-action-playback-mobile.png)、[PC 行动结果](screenshots/conquest-action-playback-desktop.png)、[原点数与整数行军次数](screenshots/conquest-original-points-mobile.png)、[零兵力君主](screenshots/conquest-zero-army-ruler.png)。

## 验证与证据

先记录失败场景并运行失败测试，再实现。旧版存档通过 v1.33.3 的正常模块接口生成，保留生成来源与 SHA-256；规划、暂停和执行途中分别验证。点数对照检查包括 3.2 起始值、原费用、免费调税，以及剩余 0.2 点结转后成为 3.3 点的实际结果。

随机开局结构验收覆盖四图、2～8 人、每种配置 32 个上下文，共 896 个开局；验证不同布局、互不相邻、资源一致、相同上下文重放一致与恢复位置不变。完整电脑矩阵覆盖四图 × 2～8 人 × 三档难度 × 两种迷雾，共 168 局，保留完整压缩重放和源码摘要；最终 168 局均结束，耗时 1～53 轮，共 132,552 个动作／计时步骤。电脑僵持回归使用自然完整对局中的真实状态，不增加资源或指定赢家。

28 场完整 HTTP 对局覆盖四图各人数，使用真实 HTTP、SQLite 和生产规则接口，并逐步核对重放摘要。为避免慢动作使长矩阵只在等待，长对局夹具推进单调模块测试时钟，记录未变换的模块上下文和信封；动作、资源、出生点与结果不改。测试确认动画原始时长及玩家规划时间仍保持。浏览器、并发检查和独立真人席位托管＋电脑整局使用真实速度。

浏览器验收覆盖四图、PC、320／390px、全屏、鼠标与触屏、半数／精确人数、金币与人口限制、行动播放、减少动态效果、重连和零兵力君主。行动播放保留逐帧文本、状态、位置与移动轨迹；检查执行和结果各有可辨识的停留时间，镜头恢复且历史不重播。触屏定位在真实绘制区域保留至少 24px 命中余量，镜头准备不得下单或改变选区。

前端针对性检查通过 63 项；另外保留 8 项小领地／零兵力君主、6 项逐步播放、28 项人数控件与 27 项全屏检查。

完整普通交互回归通过 913 项、213 条原生输入与 98 组逐帧测量。原生输入条数会随随机开局所需的定位手势变化；以当次结果为准。浏览器为 Chromium，手机尺寸与触摸为模拟，不作为实体设备或 WebKit 结论。

## 复跑

```sh
. scripts/env.sh
CONQUEST_BOT_ARTIFACT_DIR=.local/conquest-round/bots go test -count=1 -parallel=4 -timeout=30m ./games/conquest/server ./games/conquest/maps
go test -count=1 -parallel=4 -timeout=45m ./games/conquest/tests
WIRE_BOARD_CONQUEST_SCHEDULED=1 go test -count=1 -timeout=30m -run '^TestConquestAutoPlayMixedCompleteHTTP$' ./games/conquest/tests
npm --prefix web test -- conquest
python3 -B games/conquest/tests/accept_map_ux.py --army-touch
python3 -B games/conquest/tests/accept_map_ux.py --round-flow --skip-build
python3 -B games/conquest/tests/accept_map_ux.py --amount-controls --skip-build
python3 -B games/conquest/tests/accept_map_ux.py --fullscreen-layout --skip-build
python3 -B games/conquest/tests/accept_map_ux.py --skip-build
```

浏览器入口顺序执行，不与另一轮浏览器验收或前端构建并行。最终产物归档在主工作区 `.local/conquest-round-delivery/`，仅清理本轮临时构建、缓存、浏览器目录与工作树。发布复用同一提交在 main 上通过的构建，由版本 tag 触发；部署后核对公网版本、健康及既有上下架设置。
