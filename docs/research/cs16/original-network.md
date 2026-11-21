# CS 1.6 原版网络方案的源码证据

GoldSrc／CS 1.6 使用客户端命令与服务器权威状态同步：客户端发送 `usercmd`，服务器返回本人的 `clientdata`、可见实体状态增量及事件。客户端预测本人运动，再按权威结果校正；其他实体主要依赖服务器状态。**它不是双向只传按键的确定性锁步，也不是每次发送完整存档。**

本次只读取源码和分发归档配置，没有运行、抓包或反编译原始 GoldSrc 引擎。全部资料位于 `.local/cs16/research/network-reference/`，逐文件 URL、字节数、SHA-256 和 Git blob SHA-1 保存在 `sources.json`；结论与精确代码范围保存在 `findings.json`。当前项目的 53 KB 是内部存档样本，不能当作原版或现版网络包大小。

## 证据来源与边界

| 来源 | 固定提交 | 能直接证明的范围 |
| --- | --- | --- |
| Valve 官方 [Half-Life SDK](https://github.com/ValveSoftware/halflife/tree/b1b5cf5892918535619b2937bb927e46cb097ba1) | `b1b5cf5892918535619b2937bb927e46cb097ba1` | 官方命令／实体结构、客户端预测入口、游戏 DLL 的可见性与条件编码接口；不是完整 GoldSrc 引擎源码，也不是 CS 游戏 DLL 源码 |
| 第三方 [ReHLDS](https://github.com/rehlds/ReHLDS/tree/8305936010ccd038af7665a17527df5c53965f58) | `8305936010ccd038af7665a17527df5c53965f58` | 兼容服务端引擎的包处理、增量基线、位编码和 netchan；项目自述基于原 HLDS 6152／6153 的 DWARF 逆向，并包含修复和扩展，不能将所有分支等同未修改原版 |
| 第三方 CS 分发归档的 [delta.lst](https://github.com/martinsam16/cstrike/blob/651d0d248ba04028b1b8167b156888b522bbcb73/opt/hlds/cstrike/delta.lst) | `651d0d248ba04028b1b8167b156888b522bbcb73` | 该实际归档样本声明的字段、位宽和倍率；未建立 Valve 官方签名或发行版真实性 |
| 已保存的 [ReGameDLL_CS](https://github.com/rehlds/ReGameDLL_CS/tree/1f16a7f25c14225931f8df1abc51f68d18ded839) | `1f16a7f25c14225931f8df1abc51f68d18ded839` | CS 兼容游戏 DLL 的投影与当前 delta 配置；含扩展，参数与分发归档确有差异 |

官方 SDK 接口和兼容引擎实现互相印证整体设计。精确包尺寸、某个 Steam 发行构建的逐字节行为及其全部默认设置，仍需要对应原始二进制与实际网络捕获，不能从这些源码直接得出。

## 客户端发送什么

官方 [`usercmd_t`](https://github.com/ValveSoftware/halflife/blob/b1b5cf5892918535619b2937bb927e46cb097ba1/common/usercmd.h#L23-L40) 包含命令时长 `msec`、插值时间 `lerp_msec`、视角、前后／左右／上下移动量、按钮位、impulse 等。它描述操作意图和模拟时间，不只是键盘按下／松开事件；服务端独立运行运动与游戏规则。

结构体成员不等于每个模组实际传输字段。例如 SDK 结构中有 `weaponselect`，本次 CS 分发样本的 `usercmd_t` delta 表却没有它；该表包含 `impact_index` 与 `impact_position`。因此也不能笼统宣称所有 usercmd 都完全没有坐标字段，更不能把辅助 impact 数据当作服务器认可的角色最终位置或伤害结算。

ReHLDS 的 [`MSG_WriteUsercmd`／`MSG_ReadUsercmd`](https://github.com/rehlds/ReHLDS/blob/8305936010ccd038af7665a17527df5c53965f58/rehlds/engine/common.cpp#L323-L331) 使用 `usercmd_t` 注册表进行位级增量编码；读取入口在同文件 [1130～1141 行](https://github.com/rehlds/ReHLDS/blob/8305936010ccd038af7665a17527df5c53965f58/rehlds/engine/common.cpp#L1130-L1141)。[`SV_ParseMove`](https://github.com/rehlds/ReHLDS/blob/8305936010ccd038af7665a17527df5c53965f58/rehlds/engine/sv_user.cpp#L1565-L1674) 读取 `numbackup` 与 `numcmds`，最旧命令先解码，下一条相对前一条，再依序运行新命令。协议可携带有限旧命令冗余以补丢包，不把每条移动命令当必须可靠重传的事务。

购买、聊天、武器指令等还可以通过 `clc_stringcmd` 交给游戏 DLL；[`SV_ParseStringCommand`](https://github.com/rehlds/ReHLDS/blob/8305936010ccd038af7665a17527df5c53965f58/rehlds/engine/sv_user.cpp#L1445-L1470) 证明客户端上行并非只有移动包。兼容 CS 游戏 DLL 的 [`InternalCommand`](https://github.com/rehlds/ReGameDLL_CS/blob/1f16a7f25c14225931f8df1abc51f68d18ded839/regamedll/dlls/client.cpp#L2654-L2678) 处理聊天，同文件 [3556～3616 行](https://github.com/rehlds/ReGameDLL_CS/blob/1f16a7f25c14225931f8df1abc51f68d18ded839/regamedll/dlls/client.cpp#L3556-L3616) 处理 `weapon_`、弹药和购买命令；具体命令的发送可靠性不能仅由服务端解析入口推定。

官方 [`HUD_PlayerMove`](https://github.com/ValveSoftware/halflife/blob/b1b5cf5892918535619b2937bb927e46cb097ba1/cl_dll/cdll_int.cpp#L138-L143) 调用共享 `PM_Move`；[`HUD_PostRunCmd`](https://github.com/ValveSoftware/halflife/blob/b1b5cf5892918535619b2937bb927e46cb097ba1/cl_dll/hl/hl_weapons.cpp#L1047-L1075) 还区分同一命令首次预测与重放，避免重复声音／特效。这是本地预测加权威校正的证据，不能据此推导所有客户端都完整执行并知道整场私有规则。

## 服务器返回什么

[`SV_SendClientDatagram`](https://github.com/rehlds/ReHLDS/blob/8305936010ccd038af7665a17527df5c53965f58/rehlds/engine/sv_main.cpp#L5073-L5135) 每次组装游戏时间、`SV_WriteClientdataToMessage`、`SV_WriteEntitiesToClient` 和本端 datagram，再交给 netchan。它没有把其他人的 usercmd 当作主要下行广播内容。

| 下行内容 | 实现与实际含义 |
| --- | --- |
| 本人权威状态 | [`SV_WriteClientdataToMessage`](https://github.com/rehlds/ReHLDS/blob/8305936010ccd038af7665a17527df5c53965f58/rehlds/engine/sv_main.cpp#L1284-L1399) 生成 `svc_clientdata`，本人运动、生命、视角相关数据按字段 delta；武器预测数据仅在 `DELTA_CheckDelta` 判断变化后发送对应武器槽 |
| 可见实体状态 | [`SV_WriteEntitiesToClient`](https://github.com/rehlds/ReHLDS/blob/8305936010ccd038af7665a17527df5c53965f58/rehlds/engine/sv_main.cpp#L4931-L5059) 先设置本接收者可见集合，再调用游戏 DLL 的 `AddToFullPack`；玩家、门、投射物等是经过筛选的实体状态 |
| 实体新增／变更／移除 | [`SV_CreatePacketEntities_internal`](https://github.com/rehlds/ReHLDS/blob/8305936010ccd038af7665a17527df5c53965f58/rehlds/engine/sv_main.cpp#L4620-L4802) 生成 `svc_packetentities` 或 `svc_deltapacketentities`，按实体 ID 合并旧／新集合；相同实体未变化可不写字段，新进入实体从实体 baseline 编码，离开集合发送 remove |
| 事件与游戏消息 | 同次实体发送还调用 `SV_EmitEvents`；SDK 与引擎提供独立可靠／非可靠消息目标。事件不是持久化历史环的整段重发 |

“FullPack”是游戏 DLL 填写的候选网络投影，不代表它整个 C 结构体原样上网；之后还会经过实体 delta 和位编码。“无 delta”也不表示发送完整存档：它发送当前可见实体集合，并可相对初始化的实体 baseline 编码。

## 基线确认、字段位图与量化

这里有两类确认，不能混为一谈。

1. **网络包／可靠通道确认。** netchan 记录 packet sequence、ACK 和可靠序号位，处理重复包、乱序与可靠载荷重发。
2. **可用状态基线声明。** 客户端用 `clc_delta` 告诉服务端希望相对哪个已拥有的快照序号编码；[`SV_ParseDelta`](https://github.com/rehlds/ReHLDS/blob/8305936010ccd038af7665a17527df5c53965f58/rehlds/engine/sv_user.cpp#L1473-L1476) 读取该字节，`SV_CreatePacketEntities_internal` 从本客户端 frame 环取得它，并在 `svc_deltapacketentities` 里写回基线序号。本人 `clientdata` 也使用对应基线。

[`SV_ExecuteClientMessage`](https://github.com/rehlds/ReHLDS/blob/8305936010ccd038af7665a17527df5c53965f58/rehlds/engine/sv_user.cpp#L1805-L1827) 在解析本次客户端消息前将 `delta_sequence` 重置为 -1；没有可用 delta 声明时走 `svc_packetentities`／零基态 clientdata，而不是盲目假定对方收到上一包。本次没有取得官方客户端引擎的完整解码源码，不能仅据服务端代码声称已穷尽客户端丢包、基线过期与恢复的所有分支。

[`DELTA_MarkSendFields`](https://github.com/rehlds/ReHLDS/blob/8305936010ccd038af7665a17527df5c53965f58/rehlds/engine/delta.cpp#L518-L590) 比较结构字段并允许条件编码回调再删减／强制字段；[`_DELTA_WriteDelta`](https://github.com/rehlds/ReHLDS/blob/8305936010ccd038af7665a17527df5c53965f58/rehlds/engine/delta.cpp#L838-L862) 写短位图，再只写标记字段。[`DELTA_WriteMarkedFields`](https://github.com/rehlds/ReHLDS/blob/8305936010ccd038af7665a17527df5c53965f58/rehlds/engine/delta.cpp#L622-L712) 根据类型、倍率与 `significant_bits` 写位，而非反复写 JSON 字段名、十进制数字和对象结构。这里的 delta 多数是“仅发送变化字段的当前值”，不应一概理解为对每个数值发送算术差。

字段精度随模组／版本而异，不能直接套用当前 ReGameDLL 的表冒充原版：

| 字段示例 | 固定 CS 分发样本 | 固定 ReGameDLL 当前配置 |
| --- | --- | --- |
| 本人 `clientdata.origin` | 21 位有符号，倍率 128，量化步长 1／128 原始单位 | 24 位有符号，倍率 1024，步长 1／1024 |
| 玩家实体 `origin` | 18 位有符号，倍率 32，步长 1／32 | 24 位有符号，倍率 32，步长 1／32 |
| 普通实体 `origin` | 16 位有符号，倍率 8，步长 1／8 | 16 位有符号，倍率 8，步长 1／8 |
| usercmd 按钮／时长／视角 | 按钮 16 位，msec 8 位，各视角 16 位角度编码 | 对应示例相同 |

样本位置见 [clientdata](https://github.com/martinsam16/cstrike/blob/651d0d248ba04028b1b8167b156888b522bbcb73/opt/hlds/cstrike/delta.lst#L7-L17)、[玩家实体](https://github.com/martinsam16/cstrike/blob/651d0d248ba04028b1b8167b156888b522bbcb73/opt/hlds/cstrike/delta.lst#L127-L138)、[usercmd](https://github.com/martinsam16/cstrike/blob/651d0d248ba04028b1b8167b156888b522bbcb73/opt/hlds/cstrike/delta.lst#L205-L222)；对照 [ReGameDLL delta.lst](https://github.com/rehlds/ReGameDLL_CS/blob/1f16a7f25c14225931f8df1abc51f68d18ded839/dist/delta.lst)。这些是该表的字段声明，不是本次原版网络抓包测量。

## 先裁剪，再编码

官方 [`SetupVisibility`／`AddToFullPack`](https://github.com/ValveSoftware/halflife/blob/b1b5cf5892918535619b2937bb927e46cb097ba1/dlls/client.cpp#L1058-L1155) 从接收者视点建立 PVS／PAS，排除不在集合的实体、无模型实体、NODRAW 和其他观战者；预测中的本地实体还可用 `FL_SKIPLOCALHOST` 省略。PVS 是潜在可见叶集合，不等同“摄像机正前方”或逐物体精确视线，也不能直接当成完整反透视保证。

官方 [`Entity_Encode`／`Player_Encode`](https://github.com/ValveSoftware/halflife/blob/b1b5cf5892918535619b2937bb927e46cb097ba1/dlls/client.cpp#L1384-L1488) 会剔除本人实体的 origin，因为更高精度的位置已走 `clientdata`；跟随父实体的对象也可省略重复位置。固定 ReGameDLL 的 [`AddToFullPack`](https://github.com/rehlds/ReGameDLL_CS/blob/1f16a7f25c14225931f8df1abc51f68d18ded839/regamedll/dlls/client.cpp#L4542-L4660) 保留对应 CS 投影接口，但扩展分支与 PVS 缓存行为须按第三方版本看待。

## 可靠与非可靠数据分流

ReHLDS [`NET_IPSocket`](https://github.com/rehlds/ReHLDS/blob/8305936010ccd038af7665a17527df5c53965f58/rehlds/engine/net_ws.cpp#L1466-L1468) 创建 `SOCK_DGRAM／IPPROTO_UDP` socket；可靠性由 netchan 在其上提供。SDK 的 [消息目标常量](https://github.com/ValveSoftware/halflife/blob/b1b5cf5892918535619b2937bb927e46cb097ba1/common/const.h#L571-L579) 明确区分 `MSG_ONE`／`MSG_ALL`、`MSG_ONE_UNRELIABLE`／`MSG_BROADCAST`，并有 PVS／PAS 对应可靠目标。

[`WriteDest_Parm`](https://github.com/rehlds/ReHLDS/blob/8305936010ccd038af7665a17527df5c53965f58/rehlds/engine/pr_cmds.cpp#L2071-L2101) 将可靠单播写入本连接 `netchan.message`，非可靠单播写入 `datagram`，初始化写入 `signon`。[`Netchan_Transmit`](https://github.com/rehlds/ReHLDS/blob/8305936010ccd038af7665a17527df5c53965f58/rehlds/engine/net_chan.cpp#L297-L310) 能将可靠载荷、分片与非可靠载荷合在一包；[463～520 行](https://github.com/rehlds/ReHLDS/blob/8305936010ccd038af7665a17527df5c53965f58/rehlds/engine/net_chan.cpp#L463-L520) 写序号／可靠位，优先放可靠数据，放不下时可丢掉本次非可靠载荷。[`Netchan_Process`](https://github.com/rehlds/ReHLDS/blob/8305936010ccd038af7665a17527df5c53965f58/rehlds/engine/net_chan.cpp#L767-L812) 不等待丢失的旧包才使用新包，并按可靠确认清理可靠缓存。

这使频繁状态更新可以被更新状态替代，同时让必须到达的初始化或特定游戏消息可靠交付。可靠消息自身仍可能排队，不能概括成“整个 UDP 协议完全没有阻塞”。当前平台采用 WebSocket，状态差分与量化可直接借鉴，UDP 的丢包冗余和可靠子流不能照搬为现有传输已经具备的能力。

## 哪些内容只在初始化或恢复时补齐

| 内容 | 发送时机与边界 |
| --- | --- |
| delta 字段描述 | [`SV_WriteDeltaDescriptionsToClient`](https://github.com/rehlds/ReHLDS/blob/8305936010ccd038af7665a17527df5c53965f58/rehlds/engine/sv_main.cpp#L1045-L1070) 在 serverinfo 流程发送模式描述，不随每个实体反复附字段名 |
| 资源目录与索引 | [`SV_SendResources_internal`](https://github.com/rehlds/ReHLDS/blob/8305936010ccd038af7665a17527df5c53965f58/rehlds/engine/sv_main.cpp#L1232-L1281) 发送资源类型、文件名、索引、大小等，可给独立下载地址；地图／模型正文不是常规动态快照成员 |
| 地图实体 baseline | [`SV_CreateBaseline`](https://github.com/rehlds/ReHLDS/blob/8305936010ccd038af7665a17527df5c53965f58/rehlds/engine/sv_main.cpp#L6018-L6046) 写入 signon，客户端 spawn 时 [发送 signon](https://github.com/rehlds/ReHLDS/blob/8305936010ccd038af7665a17527df5c53965f58/rehlds/engine/sv_main.cpp#L1705-L1717)；后续新进入可见集合的实体可再次相对此 baseline 编码，但不重发模型文件 |
| 当前可见实体集合与本人状态 | 初始或没有可用快照基线时发送无帧 delta 的可见集合／本人状态；正常运行继续发送变化字段，不能只在初始化发送位置、生命或弹药 |
| 换图／重新接入 | 重新握手并建立对应地图资源与 baseline；普通实体进入／离开 PVS 使用实体新增／移除，不重发整个资源库、存档或历史事件 |

发送频率也不是写死 33 Hz。兼容引擎读取 [`cl_updaterate`](https://github.com/rehlds/ReHLDS/blob/8305936010ccd038af7665a17527df5c53965f58/rehlds/engine/sv_main.cpp#L5423-L5442)，由服务器上下限约束，并按 [`next_messagetime` 与可发送条件](https://github.com/rehlds/ReHLDS/blob/8305936010ccd038af7665a17527df5c53965f58/rehlds/engine/sv_main.cpp#L5284-L5343) 调度。因此“网络约 33 Hz”只能描述当前项目的配置与实测，不能当作与原版等价的证明。

## 对当前实现的直接启示

以下是据源码得出的实现建议，不是本次观察到的原版运行结果。保留服务器权威、本人预测和必要校正；将调查开始时约 33 Hz、P95 约 4.9 KB 的动态 JSON 收敛成按接收者与基线裁剪的状态差分。

1. 固定可复用的字段模式、实体 ID 与物品索引，把资源目录、固定选项和角色定义移出常规帧。
2. 对玩家、门、投掷物和目标分别维护新增／修改／移除，以字段位图仅编码有变化的必要状态；本人私有状态与远端绘制状态分开，避免重复 origin 和整份库存。
3. 明确 `baseRevision` 与当前 revision，保留有界基线并处理缺口／重连／换席；“socket 写成功”与“客户端拥有可用解码基线”在契约上区分。
4. 以当前定点规则精度为边界选择位宽，保持模拟与显示一致；不要为了模仿某份配置而降低碰撞或命中精度，也不要将所有内部高精度字段都逐帧传给远端。
5. 在既有 WebSocket 上先验证精简／差分与恢复，不把切换传输协议当作减小 JSON 投影的前置条件；继续测原生输入到像素、实际 wire、慢端和十真人容量。

本次不能给出“原版每包固定多少字节”或“修改后必然达到多少带宽”的数字。原版包长随可见实体、变化字段、事件、玩家配置、版本和网络条件改变；需要真实抓包与同局面对照才能量化。
