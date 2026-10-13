# 星垒扩展战场协议 v2

保留 v1 字段。`map` 现在来自每局保存的世界，坐标始终不平移，旧塔与建造点 ID 永不改变。schema 1 自动迁移到 schema 2，正在战斗的旧敌人保持原道路。

- `map`: `{width,height,path,pads,base,routes,regions,features}`；path 为原主路的兼容字段。`base:{x,y}`。
- `pads`: `{id,x,y,terrain:"plain"|"highground",sector:number}`。
- `routes`: `{id:string,label:string,sector:number,points:Point[],active:boolean}`。所有敌人走保存的 `route`，不根据 active 重新分配；active 只影响新出生敌人。
- `regions`: `{id:number,x,y,width,height,biome:"forest"|"canyon"|"river"|"snow"|"ruins"|"marsh",label:string}`。
- `features`: `{id:string,kind:"bridge"|"river"|"highground"|"mine"|"beacon"|"convoy",x,y,width,height,sector:number}`。
- `world`: `{seed:number,stage:1|2|3|4,preparedWave:number,map:Map,choice:Choice|null,boonChoice:Choice|null,boons:{power:number,reach:number,fortune:number},event:WaveEvent,forecast:WaveEvent[],objectives:Objective[],branches:string[],shortcutWave:number}`。
- `Choice`: `{kind:"route"|"boon",stage:number,title:string,options:[{id,label,description,map?:Map}],votes:string[],deadline:number}`。votes 按席位排列，空字符串表示未投；一致票可立即确定，否则全员投票后多数决定；平票按 options 顺序，超时按已投票多数／首选项。路线选项附完整预览地图。只在 prepare 可投票。所有选择确定后才可 ready。扩展在 6/11/16 波准备期选择；确定后重新给足准备时间。每守过 5 波提供全队成长选择。
- `WaveEvent`: `{id:"calm"|"fog"|"frost"|"surge"|"starlight",label,description,wave:number}`；forecast 为后两波预告。浓雾射程 -20%，寒潮敌速 -15%，多路突袭开放所有道路，星辉法师伤害 +20%。
- `Objective`: `{id,kind:"mine"|"beacon"|"convoy",label,description,sector,x,y,cost,status:"available"|"active"|"completed"|"failed",startWave,endWave,reward,baseAtStart,hp,maxHP,route,progress}`。玩家在 prepare 完成团队选择后支付个人金币承接；矿场单人 60 金币、多人 120 金币，撑过一波且目标存活，每人奖励 90 金币；瞭望塔 160 金币，撑过一波后全队永久增加 8% 射程；补给车 100 金币，以 90 像素／秒沿保存路线移动，抵达基地修复 4 点生命且每人奖励 60 金币。附近敌人会伤害目标，HP 归零失败；矿场／瞭望塔有 120 HP，补给车 100 HP。补给车无固定波数截止（endWave=0），暂停／准备时不移动；对局结束未送达则明确失败。基地在远处受损不会直接判定支线失败。
- 成长 `boons` 是叠加次数：每次 power 增加 12% 伤害、reach 增加 8% 射程、fortune 增加 10% 击杀／过波金币。高台再增加 15% 射程。

- `tower.priority`: `"first"|"strong"|"support"`，默认 first；effectiveRange/Damage/Interval 包含地形、队伍成长与波次效果。
- `enemy.route`: string；`phase`: number (boss 1/2)，`abilityTick`: number。新增 kind `healer|banner|splitter|engineer`。
- 新动作：`{type:"route_vote"|"boon_vote",choice:string,epoch}`；`{type:"objective",objectiveId:string,epoch}`；`{type:"target",towerId,priority:"first"|"strong"|"support",epoch}`。
- `controls.canRouteVote`, `canBoonVote`；`controls.objectives:[{objectiveId,cost,enabled}]`，`controls.targets:[{towerId,enabled}]`。`canReady` 为仍有未决选择时 false。
- 新事件 type `expand|boon|objective|objective_hit|heal|summon|boss_phase|shortcut`，与旧特效一样，重连不重播。

后端内部协作：State.World World；State.mapData() Map；routePath(id) []Point；pathDistance([]Point)、pathPosition([]Point,d)；towerStats(Tower)；worldPrepare(now) 建立预览／事件／轮换；resolveChoices(now) 超时完成投票；onWaveCleared() 处理目标。Enemy.Route/Phase/AbilityTick 为保存字段。

初始林地从三个确定模板选择北侧、林间或南侧入口，原存档保留完全原始道路。第 6 波 canyon/river、第 11 波 snow/marsh、第 16 波 ruins/forest；地图逻辑尺寸依次 960×640、1440×640、1440×860、1920×960，建造点 16/24/32/40。每阶段新增路线在不同位置汇入基地，分别跳过原防线的不同部分。所有路线的河流交点提供实际桥梁。
