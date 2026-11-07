# 完整对战版实施契约

用户已明确要求完整对战版，完成后合并、发布并更新服务器。范围为 PC 1.0.0.1051 的全部常规植物（49，含模仿者）、全部僵尸原型（33，含植物僵尸与红眼巨人），以及白天、夜晚、泳池、浓雾、屋顶与夜间屋顶。单机冒险、迷你游戏关卡、生存模式与禅境花园不在本次范围。原表中 4 个单机专用植物原型保留证据，不冒充常规可选植物。

## 稳定协议

- `Options` 增加 `scene: day|night|pool|fog|roof|roofnight`，默认 day；其余 mode/teams/roundSeconds/prepSeconds 不变。
- `View` 增加 `map:{id,rows,cols:9,night,waterRows:number[],roof,fog}`。草地 5 行，泳池／浓雾 6 行，屋顶 5 行。逻辑坐标仍每格 1000；Y 使用行内同样的逻辑单位，绘制映射至原图。
- `Player` 增加 `deck:string[]`（至多 10、不能重复）、`imitaterKind:string`，默认按场景提供可玩卡组。`deck` 动作 `{type:"deck",kinds:string[],imitateKind?:string,epoch,seq}` 仅布阵阶段可用，换卡不能清除冷却或退款。
- `plant`／`spawn` 保留原动作，模仿者增加 `imitateKind`，蹦极等格子目标允许 row+col。炮弹主动技能使用 `{type:"ability",plantId,row,col,epoch,seq}`。所有消费、归属与合法性由服务端决定。
- `Spec` 增加 `target:cell|row|plant|area`、`layer:support|body|shell|instant`、`upgradeFrom?:string`、`waterOnly?:bool`、`roofOnly?:bool`、`scenes?:string[]` 等公开规则元数据；不让 UI 推测合法性。
- `Plant` 增加 `layer,asleep,state,targetKind,imitated,uses,targetId,effectUntil,chargeMS`；花盆／睡莲、本体、南瓜可同格，铲子按 plantId 只操作本人一层。咖啡豆属于瞬时同格唤醒；升级要求本人原植物，玉米加农炮要求相邻两株本人玉米投手。
- `Zombie` 保留原字段并增加 `allegiance,altitude,diving,digging,targetId,abilityMS,abilityUntil,flags:map[string]int64`。单位转换、飞行、潜水、地底和车辆必须是规则状态。
- `Projectile` 支持二维逻辑 `x,y,vx,vy,targetX,targetY,landAt,height,radius,pierce,payload,targetId`，覆盖抛物线、星星、追踪弹、玉米黄油、冰西瓜与玉米炮。
- `State` 增加地形、墓碑、梯子、拾取物和 `fogClearedUntil`；所有随机和计时状态进入存档、Clone、Validate 和摘要。

## 分工边界

根 agent 维护公共类型、动作与放置／升级、场景地形、合法性、存档／视图、同步、共享伤害与生命周期、总时钟和平台发布。植物 agent 维护 `engine/plants.go`、`engine/projectiles.go` 与相应先行 E2E；僵尸 agent 维护 `engine/zombies.go` 与相应先行 E2E。双方不得覆盖公共类型，缺字段先反馈。资源 agent 维护素材脚本、classic-v2、原版取证；UI agent 维护 web（runtime.ts 除外）和浏览器测试。

## 浓雾与预测

普通场景继续确定性操作重演。浓雾按认证 viewer 投影：植物方看不到未被路灯／三叶草揭示的敌方单位和相关私有事件；僵尸方可以看到自己的单位。恢复快照和同步都不得夹带隐藏对象。

浓雾使用 `wireMode:"projection"` 的必要权威投影更新与本地有限预测；公开快照标记 `projection:true`，不得作为服务端存档接受。只对本人合法操作、公开运动和效果预测，隐藏碰撞与伤害由服务器确认。其他场景使用 `wireMode:"replay"`。`visibility.cells` 供 UI 绘制浓雾，不能靠透明遮罩代替信息过滤。

## 完成标准

每个可选单位均有独立可核验的规则、动画、场景适配和电脑合法使用策略。开放场景须覆盖 1v1／2v2、整局、换边、托管、重启、真实浏览器输入、手机与全屏。先列失败场景、先写 E2E，再实现。没有通过的内容继续完成，不能用目录数量、原型截图或「第一阶段」作为交付完成依据。
