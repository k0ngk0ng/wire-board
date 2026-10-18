# 战役半场确认接线

默认半场确认现已恢复原敌方选人、菜单清理、角色提交、等级重算和四个人物更新。只在带战役准备快照的运行时收到新的 Start 确认时执行；等待、按住 Start、开场继续推进和存档恢复不会重复抽取阵容。

后续引擎已加入可保存的己方换人、装备／卸装和战术编辑事务，见 [准备编辑验证](CAMPAIGN_PREPARATION_VERIFICATION.md)。本报告保留默认确认的源对照范围；相关服务器与界面仍待接入。

## 实现与源边界

`confirmCampaignIntermission` 使用当前已提交的己方角色、装备和战术。外层原随机循环与成员确认的一次随机调用分开执行。原 `89AC` 菜单清理清除 Kind5 和九个菜单工作字段；这些字段在比赛中与场地状态共用，操作仅发生在准备阶段。

`commitOriginalCampaignRoster` 对应 `B2EF..B32D`：先从四个待提交角色缓存复制角色，再重算等级；对对象 0～5 清动作及 X 速度／位置的高字节，保留低字节；清前六个 Passages 和规定的结果别名，进入 Scene1／phase0，最后只改球投影 Y 的低字节。原 `B32D..B337` 随后依次执行四次人物初始化、动画、运动和投影，复用已有 `stepOriginalResultActor`，没有将其当作纯绘制跳过。

配置角色与阶段角色在同一事务中提交。进入 Playing 后由新角色重建属性和装备上下文，同时继续合并原 `Carry`。比分、Period、WinBits、当前装备和所有保留的世界状态不会通过新建 Runtime 重置。验证器拒绝缺少 Carry、存在结果或使用偶数 Period 的战役半场状态。

原菜单选择和稍后的角色提交之间有等待／战术界面；本平台确认省略这些界面等待，不把两段独立原调用冒充跨越该间隔的连续 RNG 轨迹。源证据范围见 [原确认夹具说明](CAMPAIGN_CONFIRMATION_SOURCE.md)。

## 测试与结果

先列失败场景、写测试并保留缺失 API 的编译失败，再编写实现。记录在工作树 `.local/campaign-intermission/before.log`、`before-complete.log`。

- 四条原菜单来源同时覆盖单人、合作、装备主场和真实半场。提交核心与四人物尾段分别将整个 `OpeningState` 与原 RAM 解码结果比较，全部一致；字段夹具保留写入顺序、源摘要和采样限制。
- 真实单人半场保留 15:9 比分、Period3 和完整 Carry。测试等待数帧后确认，敌方由槽 `[0,3]` 改为 `[0,2]`，新姓名 HUD、角色和等级同步。连续按住 Start 与恢复后的逐帧状态、显示、声音保持一致。
- 原完整比赛测试曾假设两半场角色不变。新源证据推翻该假设，首次失败保留在 `campaign-regression-first.log`；测试现分别检查首半场原阵容和已确认的下半场阵容，原类型及装备检查不放宽。
- 原随机调用和尾段恢复使既有整场轨迹发生变化。当前正常单人路线 Native／Wasm 各两场均为 22,385 视频帧、7,433 逻辑帧、84 次恢复检查，比分 24:23。四次轨迹 SHA-256 均为 `06e3969e5a135ed59a8908e74194426103b1ad69cdb8f67e49ae57caeb6714ec`。此前 18:12 及旧摘要只属于历史实现。
- 结算、奖励和下一场 Court4／Period4 开局定向检查继续通过；Native／race／Wasm 的原提交、真实半场、结算及既有半场显示／保分检查均通过。篮球全部包编译及模块边界检查通过。

两项全电脑战役卡局仍在第一节出现，原失败没有被隐藏或判为通过。它们与此次半场确认属于不同路径，原物理处理策略仍等待用户选择。本轮未重跑所有游戏规则，也未宣称篮球全套检查已转绿。

## 重复验证

```sh
source scripts/env.sh
.local/board-art-env/bin/python -B scripts/basketball/verify_campaign_confirmation.py
go test ./games/basketball/engine -run '^(TestCampaignConfirmation|TestOriginalCampaignRosterCommit|TestCampaignIntermission|TestCampaignSettlement|TestRuntimeIntermission|TestRuntimePeriodBoundary)' -count=1 -v
go test -race ./games/basketball/engine -run '^(TestCampaignConfirmation|TestOriginalCampaignRosterCommit|TestCampaignIntermission|TestCampaignSettlement|TestRuntimeIntermission|TestRuntimePeriodBoundary)' -count=1
GOOS=js GOARCH=wasm go test ./games/basketball/engine -run '^(TestCampaignConfirmation|TestOriginalCampaignRosterCommit|TestCampaignIntermission|TestCampaignSettlement|TestRuntimeIntermission|TestRuntimePeriodBoundary)' -count=1 -v -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec"
GOOS=js GOARCH=wasm go test ./games/basketball/engine -run '^TestCampaignPasswordEquipmentRuntimeFlow/solo$' -count=1 -v -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec"
```

本轮日志及结果索引位于工作树 `.local/campaign-intermission/`。半场主动换己方角色、装备和战术的界面与服务器协议尚未接入；完整战役、结局、联机和素材发布验收仍未完成。
