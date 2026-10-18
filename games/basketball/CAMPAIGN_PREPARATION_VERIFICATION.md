# 半场准备编辑事务

引擎现可在真实半场接受换己方角色、装备、卸装和搭档战术操作，并保存到 `RuntimeState.CampaignPreparation`。`ViewCampaignPreparation` 返回当前已接受的选择；`ApplyCampaignPreparation` 不推进视频、随机数、比赛时钟或物理状态。Start 确认后才把这些选择与已有原确认流程一起提交到下半场。

## 状态与原规则

`Config.Campaign` 表示最近一次已提交的准备快照，其中 Period／WinBits 仍是本场的偶数胜场基线。半场待确认的 Progress 只改变库存和装备，不写入实际奇数半场编号；实际窗口编号另存在 `CampaignPreparation.Period`。确认时原子替换配置、装备、角色和战术，保留比分、当前半场、控制器归属及 Carry，不能用新建比赛代替。

- 装备和卸装复用已验证的原 `91DD`／`931D` 规则，只接受拥有的库存下标。四名己方角色都可操作，不限当前上场两人；装备按角色保存，交换上场位置不转移装备。
- 原已确认的装备操作在之后返回菜单时不会整体撤销。关闭未提交的前端选择不发操作；关闭界面、JSON 恢复或托管切换不删除已经保存的准备内容。
- 每个修改请求绑定半场编号和准备版本。库存变化后，旧版本请求拒绝，防止旧下标指向另一件物品；无变化操作不增加版本。未来适配器仍须绑定比赛 epoch，并执行席位／房主权限校验。
- 战术是按队伍保存的指令，不是两名真人各一份。原菜单只允许“一名真人＋一名电脑”的队伍编辑。接口只允许修改热血队的战术，另一队值保持不变；两名真人或全电脑队伍不能新改搭档战术。已接受的选择不会因随后托管切换而被撤销。
- 恢复校验拒绝更改 Mode、胜场进度、奖励组、对方战术、非法角色及不可能的库存变化。满库存卸装沿用原丢弃行为，并通过 `DiscardedItem` 显式报告。该容量边界不代表正常战役自然拥有这些物品；保存来源与身份验证仍由平台负责。

源调查索引为共享 `.local/basketball/rea/campaign-preparation-semantics/REPORT.md`。战术资格与控制器的原路由在报告中单独列出，不能把 UI 的每席设置误当两份独立库存。

## 实现前的失败清单与验证

先写测试、保存缺少 API 的编译失败，再编写实现。红灯日志位于工作树 `.local/campaign-preparation/before.log`。

884 条已有原装备记录验证准备编辑的装备、卸装、空库存拒绝与取消不提交行为，包含 12 次原满库存丢弃。真实对局从 `020010` 装备主场进入半场后，依次卸下角色 0 的 item1、给角色 1 装备、交换上场位置并将本队战术改为 2；每次修改都经过 JSON 保存恢复，旧版本请求被拒绝。修改本身不改变原 Runtime 配置、比分、Carry、逻辑帧或 RNG。

随后通过实际 Start 进入下半场，控制器仍属于原平台角色位置，投篮装备按新角色绑定；真实比赛最终在第 22,652 视频帧以 18:26 结束，进入败局结算，并保留新装备归属。没有强制胜利、改种子或缩短时钟。

Native、Wasm 各重复两次；从半场初态、编辑操作到第二节结算的完整记录摘要均为 `12656c4ddd36df1bd518ddd2e1544ff4c7697b9bdd36bccfef4cdd5ee4c56785`。race 同样通过。未经编辑的默认流程复验仍为 24:23，原轨迹摘要 `06e3969e5a135ed59a8908e74194426103b1ad69cdb8f67e49ae57caeb6714ec` 不变。

原确认、半场显示／保分及结算到下一场的定向回归通过；篮球全部包编译和模块边界检查通过。两个既有全电脑悬空球失败未被本轮修复，也未据此宣称篮球全套验收通过。

```sh
source scripts/env.sh
go test ./games/basketball/engine -run '^TestCampaignPreparation' -count=1 -v
go test -race ./games/basketball/engine -run '^TestCampaignPreparation' -count=1 -v
GOOS=js GOARCH=wasm go test ./games/basketball/engine -run '^TestCampaignPreparation' -count=1 -v -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec"
```

日志和摘要在工作树 `.local/campaign-preparation/`。这些是源规则和引擎流程验证，不是浏览器／服务器接口验收。

## 平台接线待办

服务器尚未暴露准备操作，也未接入 epoch、席位权限及共同准备确认；前端尚无对应编辑界面。库存下标必须绑定用户看到的准备版本，不能给旧选择简单换上最新版本后提交。接受的修改应持久化，关闭弹窗只清理尚未提交的本地选择。编辑不推进帧，因此网络适配器还须正确同步同一帧上的新准备版本。

新战役入口另有 [原初始化组件](CAMPAIGN_NEW_GAME_VERIFICATION.md)。它只在明确的新建操作时抽一次奖励表；创建比赛、恢复、人数调整和败局继续不能隐式重抽。完整平台战役、结局、联机与素材发布仍未完成。
