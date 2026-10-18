# 战役中场操作接入

中场装备、换人和战术操作已接入游戏模块的请求校验、存档、观战投影和实际 Start 确认。创建房间仍使用原单场入口；本次不开放战役创建，不代表完整战役已接入。

## 请求与权限

`campaign-preparation` 请求包含当前 `epoch` 和引擎的 `preparation` 操作。操作中的 `period`、`revision` 必须与玩家看到的准备状态一致；装备操作提交库存索引，不接受直接授予物品。客户端不得把过期库存索引配上刚收到的新版本号重发。

共享队伍菜单由第一席未托管玩家控制，与原第一控制器的确认语义对应。第二席合作玩家、观战者和托管席位不能修改；游戏进行中的上下文没有房主字段，不把第一席权限称为房主权限。战术可用性使用平台当前的托管信息判断，仍遵循原版「己方一名真人、一名电脑」限制。

操作不推进视频帧、逻辑步、比赛时间或 RNG，不占用控制器输入序号。有效菜单操作刷新第一席的活动期限，不延长其他席位的期限。`AcceptStale` 仅允许仍满足独立 epoch／period／revision 条件的旧房间版本，`Apply` 再次校验；重复的旧库存操作被拒绝。

投影新增 `preparation` 和 `canPrepare`。观战者可查看准备状态；只有第一席可获得修改权限。已确认选择保存在原有 Runtime 中，关闭界面不需提交回退操作。Start 按已有原版中场流程提交装备、战术和双方阵容；服务端同时保存新的阵容选项，避免下个快照因旧阵容绑定而无法读取。

## 验证与产物

失败场景及测试先于实现，初始编译失败记录保存在工作树 `.local/campaign-host-preparation/before.log`。

| 验证 | 结果与范围 |
| --- | --- |
| 模块操作与恢复 | 4 次修改均不推进比赛；拒绝过期 epoch／period／revision、越权及混合字段；合作第二席不能修改共享菜单 |
| HTTP／SQLite | 真实登录、请求、观战和数据库重启后保留装备；实际 Start 和正常时间的服务器计时器进入下半场 |
| 竞态检查 | 上述模块及 HTTP 测试通过 `-race` |
| 关联回归 | 23 个测试结果通过，覆盖原控制器、输入队列、准备房间、席位模式、托管、恢复和视频历史 |
| 浏览器 Go/Wasm | 52 项通过，其中新增 4 次同帧修改、旧版本拒绝、音效不重播、20 批中场后完整 Runtime 回放一致、进入下半场；其余为已有同步检查 |
| 目录边界 | 20 个游戏无跨游戏实现导入 |

HTTP 测试的 Start 包装器仅安装从原密码 `020010`、原开局配置和实际 Go 比赛推进生成的中场快照。后续请求、校验、恢复和计时器均使用真实模块。它是受控中场边界验收，不是自然创建战役的整场 HTTP 验收，也不是原机与 Go 的整场比较。

浏览器检查使用实际模块导出的快照和真实 Go/Wasm。起点为视频帧 10965，中场比分 15:9；4 次准备修改保持帧号不变，20 批各 6 帧后在 11085 帧进入下半场。桌面和 320px 截图记录实际 Canvas；尚未包含准备弹窗的交互验收。

产物位置：

- 工作树 `.local/campaign-host-preparation/`：初始失败、模块／HTTP／race 日志、关联回归 JSONL 和 `host-views.json`。
- 工作树 `.local/poker/basketball/http/TestCampaignPreparationHTTPPersistence.json`：真实 HTTP／SQLite 过程记录。
- 共享 `.local/basketball/network-scene/`：浏览器 `result.json`、`desktop.png` 和 `mobile-320.png`。

复现时在工作树根目录运行；前端依赖须已安装：

```sh
source scripts/env.sh
go test ./games/basketball/server -run '^TestHostCampaignPreparation|^TestCampaignPreparationHTTP' -count=1 -v
go test -race ./games/basketball/server -run '^TestHostCampaignPreparation|^TestCampaignPreparationHTTP' -count=1 -v
python3 scripts/basketball/accept_network_scene.py
```

## 后续范围

仍需接入新战役／原密码入口、准备界面、奖励确认、失败继续和下一场选择。战役库存为共享资源，界面须显示满库存卸装可能丢弃物品的原版行为。不能重读密码覆盖已确认的装备，也不能在 Continue 时重抽奖励表。完整战役尚未完成；已有全电脑悬空球失败保持原样，不能用本次通过的接入检查宣称全部游戏验收通过。
