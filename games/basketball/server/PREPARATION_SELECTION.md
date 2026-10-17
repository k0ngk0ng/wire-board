# 等待房间的可视准备

用户要求：场地与球员在进入房间后的准备阶段选择，使用实际图片；创建房间仅选择人数与合作／对战模式。点击开始后直接进入球场，不重复原版人数、场地或选人菜单。

实现前失败场景：非房主改场地；玩家改他人角色；观众或比赛中修改；坏字段或越界角色被接受；成功修改不清准备；请求失败破坏现有选项；同队重复角色；换角色改变别队或席位输入映射；重启丢失选择；开始时未使用所选球场和球员；前端把选择放在创建弹窗；图片与角色槽不一致；图片加载失败后无法选择；320px 图片卡溢出或文字不可读。

房主可配置阵容。其他玩家仅能选择自己的角色；占用同队电脑角色时交换，另一真人占用时拒绝。房主可交换同队两人。所有选择都由服务器校验，改变选择后真人重新准备。场地／角色图片存在，不等于对应玩法已经通过验收；开发阶段支持范围由独立服务端门禁控制，剩余原版内容继续恢复。


## 当前验证记录

- 公共准备扩展：真实 HTTP、配置复验、权限、SQLite 故障回滚、nonce 重试、输入／输出切片隔离，以及既有模块生命周期的定向 race 通过。证据 `.local/module-preparation/{http,ui-render}.json`、`.local/module-preparation-race.log`。
- 篮球准备：已通过场地权限、本人／他人角色限制、电脑队友交换、真人准备清理、SQLite 重启、开始使用选定场地和人物、开赛后禁止修改。证据 `.local/basketball/engine/preparation-server-race.log`、`preparation-court-three-after.log`。
- 组件浏览器：13 项检查通过，原生鼠标／触摸、图片实载与回退、PC／320px、只读权限及实际文字对比度。证据 `.local/basketball/preparation-ui/`。
- 完整平台：登录、标准创建弹窗、等待房间图片选择、第二人加入、双方准备与开赛配置通过；创建阶段没有场地／球员面板，未入座的人类角色标为「等待玩家」。证据 `.local/basketball/platform-rooms/`。
- 新选择的全场验证：`BASKETBALL_PREPARATION_MATRIX=1 go test ./games/basketball/engine -run '^TestRuntimePreparedConfigurationsCompleteMatchAndRestore$' -count=1 -parallel=4 -v`。64 组包括球场 0–3、两队球员 0／1 的四种排列、单人／合作／对战／全电脑；每组正常上下半场、固定 seed、两次重复与周期 JSON 恢复。首轮 57 组通过，7 组缺口保留 `.local/basketball/rea/preparation-matrix/failures.json`。修复后重跑纽约全部 16 组、其他斜传首差 3 组，再重跑道具反弹 1 组，均通过；64 组现均有有效通过证据。未缩减选择范围或修改种子绕过，未把首轮失败记录改成成功。对应原函数与真实前态对照见 `PREPARED_DIAGONAL_VERIFICATION.md`、`BALL_PROP_CONTACT_VERIFICATION.md`。

界面与开局链路通过不等于所有完整比赛通过。当前开发阶段支持 4 个球场、固定两支球队的各两个已恢复人物槽；资源已导出全部 8 场／32 人，剩余玩法仍按整体复刻目标继续恢复。未发布、未上传 OSS。


真实时钟端到端补充：`BASKETBALL_FULL_HTTP=1 go test ./games/basketball/server -run '^TestBasketballHTTPCompletePreparedMatch$' -count=1 -v` 已完成。两名真实认证用户先操作，再托管、接回对方席位、以原 START 暂停并重启 SQLite 服务，随后原 AI 完成正常上下半场及结果动画；22,383 视频帧、7,435 逻辑更新，比分 64–36，实际耗时 6 分 13 秒。未缩短比赛钟或注入预定结果。记录 `.local/basketball/full-http/result.json`；这是完整 HTTP／主机验证，不能代替完整多人浏览器或所有游戏内容验收。

## 第五个球场与准备流程复验

当前开发版可在等待房间选择 Court 0–4。新增 Court 4 的 16 种阵容／控制配置首轮 12 组通过，4 组暴露原 Kind10 捕获分支；修复后这 4 组完整比赛通过，合计 80 种已开放配置均有通过证据。失败记录未覆盖或删除，仍不能据此开放其余场地和人物。

实际平台重新构建并通过登录、三种房间模式、准备区图片加载、第五球场选择保存、球员调整、第二人加入、准备及直接开赛，共 33 项检查。PC 和 320px 截图位于 `.local/basketball/platform-rooms/`，重复执行入口仍为 `TestBasketballBrowserPlatformRooms`。最新构建、浏览器和 Wasm 来源绑定日志为 `.local/basketball/engine/preparation-five-courts-{rebuild,browsers-after,package-after}.log`；它们对应后续四阶转场实现之前的源码，不替代后续改动验收。
