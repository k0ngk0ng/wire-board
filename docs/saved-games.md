# 保存退出与续玩

全平台设计与接入要求以[存档与续玩规范](../rules/saved-games.md)为准。本文记录当前接口、实现行为和验收方式。

存档是模块显式接入的能力。普通房间继续沿用原有生命周期；只有后端模块声明 `SupportsSave: true` 时显示「保存并退出」。仙剑是首个按个人长期存档接入的游戏，目前仍在开发验收，尚未正式注册；五子棋仅在测试进程开放存档能力。

## 保存

玩家通过普通房间命令提交 `save`，撤回使用 `save_cancel`。平台记录认证账号的同意；全体未离开的真人确认后，房间进入 `saved`，释放所有席位并移出活动房间列表。电脑席位不阻塞保存，真人托管席位仍须本人确认。

同意只针对当前局面。玩家行动、电脑行动、系统推进或控制状态变化会清除先前同意，避免旧确认覆盖新进度。最后一次确认、完整快照和幂等操作记录在同一 SQLite 事务提交，失败时不改变内存、席位和已确认名单。

持续推进的游戏可提供 `SaveControl`：首位真人同意后，模块暂停局面并清除计时和旧输入。确认期间只允许同意、取消保存或房主结束房间。任一原真人都能取消本次保存并恢复游戏，包括尚未同意的人。暂停和同意名单在重启后保留；写入失败时一并回滚。未接入此回调的游戏继续使用逐人撤回的方式。

保存状态保留在持久化快照中，独立于活动房间的占位和清理期限。保存不触发弃局或成绩归档；对局身份保留，续玩不能形成独立副本重复结算。

## 继续

`GET /api/saves` 只返回当前账号参与的已保存／等待续玩记录。摘要包含名称、游戏、原真人名单、保存时间和确认名单，不返回游戏快照或私有数据。`GET /api/state` 提供同一份摘要供大厅展示。

原玩家通过 `POST /api/saves/{id}/resume` 提交空对象确认继续。第一个确认者进入 `restoring`，其他原玩家仍可自由参加别的房间。已占用其他房间的账号不能确认续玩；陌生账号无法读取或加入存档，续档房间不开放陌生人观战。

单人一次确认即可恢复，不显示等待原队友的步骤。多人全体原真人确认后恢复 `playing`，沿用对局身份、规则状态和剩余逻辑时间，并重新经过公共资源加载就绪门槛。等待期间停止电脑和计时推进；已确认者可以通过「离开房间」返回大厅，未改变的存档继续保留。

## 游戏接入

模块自身的全部规则状态必须包含在 `Match.Data` 中，随机状态、脚本位置和未完成决策同样需要持久化。平台冻结逻辑时钟，不替游戏重新生成状态。实时游戏必须提供 `ResumeSaved` 清理输入队列、按键和控制租约，不能借恢复改变进度或增加时间。

`SaveControl` 仅可与 `SupportsSave` 一起注册。传入 `true` 时必须返回 `Paused: true` 且无计时器的合法快照；传入 `false` 时释放本次保存暂停并保留剩余预算。`ResumeSaved` 同样需要解除保存暂停。即时帧造成的旧版本可以提交保存确认，但不能越过开局或续档建立的操作版本下限。

恢复保留 `MatchID`，并建立新的操作版本下限，旧页面不能补交保存前的操作。数据库失败时确认和恢复均回滚；重复确认不会创建第二个可推进的实例。

## 版本与发布

游戏负责 `Match.Data` 的格式及兼容策略，可通过自身恢复回调实施校验或升级。发布前必须验证已有存档继续，具体要求见平台规范；不能将不兼容的快照重新标成当前版本。

仙剑原生快照校验构建与资源标识，目前已验证同构建的个人续档，跨原生构建兼容仍待验收。正式发布前须完成旧版本运行或显式迁移方案，不能把当前规则／存档测试写成跨版本兼容已完成。

仅用于开发的加血构建另有[测试存档隔离验收](../games/pal/DEVELOPMENT_TESTING.md)：正式／测试运行器互相拒绝对方的存档，同一测试构建可在全新运行器继续。该结果证明测试隔离有效，不代表正式版本间的迁移已通过。

## 验收

```sh
. scripts/env.sh
go test ./internal/gameapi -run '^TestSaveControlCapability$' -count=1
go test -race ./internal/server -run '^TestSavedGame(Lifecycle|PermissionsRollback|RealtimeConsent|ConcurrentResume)HTTP$' -count=1 -timeout=6m
go test -race ./internal/server -run '^TestPALOriginalSoloPrivateSaveRestartHTTP$' -count=1 -timeout=4m
npm --prefix web run build
SAVED_GAME_BROWSER=1 go test ./internal/server -run '^TestSavedGameBrowserFixture$' -count=1 -timeout=35m
```

在另一个终端执行浏览器验收：

```sh
. scripts/env.sh
python3 -B scripts/accept_saved_games.py
touch .local/saved-games/browser/stop
```

HTTP 结果位于 `.local/saved-games/`，其中 `concurrent-resume.json` 验证单进程服务内 1～3 名原玩家的并发继续／重试只恢复一个原对局，不产生结算。浏览器记录与桌面／320px 手机截图位于其 `browser/` 子目录。浏览器测试使用正式 React 页面、真实认证请求和原生点击／触摸；五子棋的存档开关仅存在于测试服务器，不改变正式游戏设置，也不代替仙剑原版玩法验收。仙剑个人存档结果另位于 `.local/pal/verification/`。
