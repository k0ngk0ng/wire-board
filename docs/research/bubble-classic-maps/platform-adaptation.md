# 经典地图平台适配

`games/bubbleparty/data/embed.go` 将同一份 `classic-2002.json` 编入服务端，不另外手写地图。helper 文件 `server/classic_maps.go` 与 `native_terrain.go` 不修改原始数据。

15×13 原图保持格点；唯一 13×11 的 Village00 在固定棋盘居中，坐标统一加 `(1,1)`，外圈为墙。`NativeObjects` 保留原始叠加对象 ID，空格 `-1`，并随爆破或推箱更新。硬障碍、地面 fields[6]、固定箱和移动箱分别映射到平台碰撞类别；方向门单独由对象字段控制，不把它画成整格墙。

多格硬障碍从锚点向右、向上展开：X 范围 `[x,x+width-1]`，Y 范围 `[y-height+1,y]`。补查原版加载器跳转表 `0x43442c`：type 0/1 指向 `0x43429d`，字节 `8b4db8...8b55f08bc22bc1403bc2` 读取高度后计算 `currentY-height+1`，随后逐行向下至锚点。完整原始 Evidence 在 `.local/bubble-mounts/rea-footprint-evidence.json`。该结果与原始图集中舱房、卡车、神殿等底部锚点一致。

平台开放 2/4/6/8 人。优先选取彼此分散的原始出生点，不足时只补入有出口的空地；不得清除或挪走原始障碍。选定后按左右地理位置分成两队，这是平台适配，不宣称原版服务器采用同一分队策略。全 112 图的原始出生点均未被正确多格碰撞覆盖，且均有足够空地容纳 8 人。

Village00 的两组原始双向坐标被加载器写入地面目标字段，但四个端点都位于 `Tile00_Static/Tree00` 下，地面为普通 Grass00。源图保持树木硬障碍，不删除树木制造可用传送入口；配对字段存在不能证明当前地图实际能触发传送。helper 保留配对传送能力，隔离测试只在显式清空端点的夹具验证以下机制。冷却、占用检查属于明确的重构参数：中心 0.15 格范围触发，目标有玩家、水泡或不可走地形时不传送；成功后 1 秒且需离开配对格才重新激活，站在出口不会反复跳。`TeleportUntil` 的非零值同时保存格内锁定状态，存档和重连不解除锁定。

方向门的离开许可为 fields[1..4]，顺序左上右下，进入检查对向。水柱只对 fields[5] 非零的方向门应用该限制；fields[5] 为零的可穿物可由水流清除。调用者应在水柱清除物件后同步 `clearNativeObject`，否则快照和后续通行将不一致。

矿车仅在地面 fields[8] 非零的轨道提速至 120 ms/格；其它骑乘不受轨道影响。fields[7] 未确证为危险，不把工厂地面设置成必死；fields[9] 是水域脚印条件，也不据此淘汰玩家。

补查原版 `0x449850` 确认传送带每逻辑步移动 fields[3]=2 像素（约 600 ms/格），fields[4] 为拉向格中心的方向 mask，不是禁止玩家进入的门。先按左上右下各 bit 向格中心归正，已归正时使用 fields[5] 出口方向；地面强制运动在主动走路之前叠加。飞碟的穿箱能力屏蔽地面强制运动。`nativeConveyorMotion` 以 StepMS/30 缩放原始像素位移，调用方沿相同碰撞规则执行，不改写玩家主动按键。证据 `ev_97a22b9ea7b344ec708b4eca17d649010609d59d0dae16c50f2efeefd4182226` 保存于 `.local/bubble-mounts/rea-conveyor-evidence.json`。

验证入口：

```sh
. scripts/env.sh
go test ./games/bubbleparty/server -run 'TestClassic|TestNative|TestMount' -count=1
```

测试先于 helper 实现编写，覆盖全量图及人数、居中边界、多格硬障碍、方向门、传送占用和防循环、矿车轨道速度及传送带出口。
