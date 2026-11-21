# CS 1.6 首版协作契约

本文件在实现前冻结接口与验收范围。游戏 ID 为 `cs16`。支持 2 至 10 人，默认 10 人、两队各 5 人；可添加电脑和主动托管。首版包含原版 24 种枪械、小刀、HE、闪光、烟雾、C4，以及 `de_dust2`、`de_inferno`、`de_aztec`、`cs_assault` 四张原图。三个爆破图和一个人质营救图均须能完成比赛。

## 数据与规则

GoldSrc 坐标保持 X、Y 水平，Z 向上。位置和速度采用原始单位的 256 倍整数；角度一圈 65536，俯仰范围 -16000 至 16000。规则候选频率为 100 Hz，每步 10 ms；客户端显示独立刷新。原版浮点轨迹与量化后的规则须保留差异，不承诺逐位等价。

`engine.Options` JSON 为 `{map,roundSeconds,maxRounds,teams,characters,friendlyFire}`，默认 `de_dust2`、120 秒、10 局且关闭误伤队友。`friendlyFire` 在创建和等待房间由房主管理，开局固定，省略时按 `false` 恢复；不通过游戏输入修改。开启后枪械与小刀队伤按所选原规则为 35%，HE 队伤按现有爆炸伤害处理；队杀扣 1 frag 和 3300 资金，资金最低为 0。自身伤害和 C4 目标爆炸保持既有规则。关闭误伤不移除队友实体、身体挡弹或正常穿透衰减。`teams` 使用 `t`/`ct`，省略时按席位交替分队。最大队伍人数为 5。角色使用原版模型 ID，T 为 terror/leet/arctic/guerilla，CT 为 urban/gsg9/sas/gign。

`engine.Input` JSON 为 `{seq,life,viewFrame,moveX,moveY,yaw,pitch,fire,secondary,jump,crouch,walk,reload,use,drop,select}`，移动范围 -1000 至 1000，选择武器使用物品 ID 字符串。所有业务字段为整数、布尔或字符串。输入包额外包含 `{type:"input",epoch}`。购买为 `{type:"buy",epoch,life,round,item}`，物品 ID 包含 24 枪械、hegrenade/flashbang/smokegrenade、vest/vesthelm/defuser、primammo/secammo。客户端不可上传位置、伤害或命中结果。

引擎对外接口由引擎代理尽快落地并通知其他代理：`New(options,players,seed)`、`QueueInput(state,seat,input)`、`Advance(state,targetFrame)`、`BotInput(state,seat)`、`ClearInput(state,seat)`、`Buy(state,seat,item)`、`MapByID(id)`、`Weapons()`。引擎状态包含 Frame、Players、Events、EventID、Finished、Options、Round、Phase、TeamScores；服务器投影不泄漏内部队列、随机状态、历史和电脑计划。

## 地图导出

资产代理负责 `scripts/cs16/`、`games/cs16/data/` 及 `.local/cs16/assets/`。来源位于 `.local/cs16/research/original/`，参考实现位于 `.local/cs16/research/regamedll-reference/`。正式客户端加载根为 `/cs16/manifest.json`。

服务端规则数据以 Go 数据包提供 JSON 字符串常量 `data.MapsJSON`，只含计算必需的地图平面、节点、碰撞 hull、实体与材质。不得包含客户端贴图、人物模型或声音。地图规则 JSON 顶层为地图数组；每张包含 `id,name,mode,planes,nodes,hulls,spawnsT,spawnsCT,bombsites,rescueZones,hostages,buyZones`。坐标乘 256；平面法线乘 1048576，dist 使用坐标同倍率。plane 项 `{x,y,z,dist}`；node 项 `{plane,front,back}`；负 child 是 GoldSrc contents，-1 空、-2 实体。`nodes` 为 hull0；`hulls` 包含站立/蹲下 clipnode 图与 root。实体区域为 `{min:{x,y,z},max:{x,y,z}}`。精确格式由资产与引擎代理协商，不能各自猜测。

公开地图绘制数据与模型、声音、WASM 存放 `.local/cs16/assets/`，清单引用文件名、字节数和 SHA-256，不进入 Git 媒体历史。浏览器可以直接解析原始 BSP30/WAD3/MDL10，也可以读取离线转换结果；视觉几何与规则数据须来自同一原图。缺失素材必须报错并保留重试。

## 服务器与视图

服务端负责 `games/cs16/server/`、平台注册及控制器声明，新增模块 `cs16.Module()`。`View` 顶层为 `{schema,epoch,serverAt,startAt,frame,mapId,rulesSHA256,options,players,events,eventId,round,phase,phaseEndsAt,teamScores,bomb,hostages,grenades,drops,finished,you,canAct}`。每玩家至少 `{seat,team,character,x,y,z,yaw,pitch,alive,life,health,armor,money,weapon,ammo,reserve,kills,deaths,receivedSeq,appliedSeq,crouch}`；他人私有库存、资金等按权限裁剪。常规和恢复投影均不发送 Map；使用 `mapId` 与 `rulesSHA256` 绑定规则身份，并复用已校验的资源缓存。动态数据使用连接事件游标。

帧与输入序号上限必须明确；离散边沿有序保留。初次加载统一就绪，断线不自动托管，重连只恢复本端。输入、权威应用、事件和生命代次分别校验。人物死亡可观战但不能继续提交攻击。

## 浏览器与验收

前端负责 `games/cs16/web/` 与对应 JS 测试。Three.js 渲染器独立于 React 更新；键鼠、Pointer Lock、手机多指和双摇杆经公共控制器生命周期合并。HUD 保留生命、护甲、弹药、资金、回合与队伍分数。地图图卡、选队和选造型在统一准备界面中呈现。声音使用平台音频接口。

开发前先写失败场景和测试，再实现。E2E 覆盖 10 席、四图完整比赛、24 枪械实际购买和射击、爆破与人质完整流程、真人电脑混合、接管、权限、断线、输入释放、快慢端并存与多房间。浏览器预算：正常网络原生键鼠到首次画面 P95 ≤33 ms、触屏 ≤50 ms、目标 60 FPS；主线程、规则耗时与协议字节单独报告。产物统一在 `.local/cs16/verification/`，保留源码摘要、复跑命令、JSON/CSV、截图和 trace。模拟触屏不声称实体手机实测。

所有新增代码禁止注释。命名、目录、定点业务计算、无直接播放器与临时路径限制接入各语言 lint。工具和测试的缓存、临时目录显式创建并限定在 `.local/cs16/`。不得修改其他游戏的文件或清理其产物。
