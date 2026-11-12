# 极品飞车模块契约 v3

v3 的实现前失败清单与预算见 [IMPLEMENTATION-PLAN.md](../../docs/research/racing-nfs/IMPLEMENTATION-PLAN.md)。本文件保留生产接口与明确的旧版兼容边界。

## 当前创建、准备与操作

创建选项为 `{track:"france"|"coastal"|"uk"|"park",laps:1|3|5,vehicles?:string[],timeOfDay?:"day"|"night"}`，默认 France、3 圈、白天。未指定的车型按席位循环使用 `p911/fm50/ldia/mcf1/ccor`，显式选择保持不变；实际人数少于房间容量时忽略未参赛席位的车型。各车保留独立性能和真实模型尺寸。

等待准备动作 `{type:"vehicle",vehicle:id}` 只修改已认证 `ctx.Actor` 的席位，拒绝额外 seat 或性能参数。比赛内不接受 vehicle 变更；旧 skin 动作仍只影响兼容外观。

新旧版本均使用统一房间、准备、控制器、托管与结算界面。正常 drive/reset/skin 协议、序号与 epoch 鉴权仍见下方兼容契约；`brakeAmount?:number` 为 0～1 模拟制动压力，省略时沿用 brake 布尔值，压力和布尔值必须一致。每份待执行输入保持自己的压力，不能被后来的输入覆盖。

`drive` 可携带 `horn?:boolean`，省略为 false。按住与松开使用同一有序输入协议，等待队列分别保存每份输入的喇叭状态；不增加独立动作、时限或权威状态写入入口。喇叭仅由已认证席位的正常驾驶输入控制，不改变运动、排名、圈数或比赛期限。

## v3 动态状态与静态规则

新路线输出 `physics=3`。Track 动态元数据为 `{version:3,id,name,geometry,width,height,roadWidth,length}`；geometry 是源道路完整 64 位摘要，动态 View 不重复携带 points/road/barriers/props。客户端在统一就绪门槛前加载并核验同版本道路和原生三维场景。

服务端仅嵌入必要数值规则：`points:{x,y}[]`；`road:{height,left,right,nx,ny,nz,fx,fy,fz,rx,ry,rz,surface,segment}[]`；`barriers:{kind,a:{x,y},b:{x,y},bottom,top,segment}[]`。长度均为分米，法线和方向向量保持源值，不重采样中心点。规则文件按完整 hash 不可变保存；新存档只记录精确 geometry 引用，恢复按该引用加载规则。GLB、纹理、音频和客户端运行时不进入服务器规则数据。

每辆车额外公开 `vehicle/rpm/gear/height/pitch/roll/routeIndex/motion/protected?/horn?`。gear 为 -1 或 1～原车自动挡数量；pitch 上坡为正，roll 以模型局部 +X 侧抬高为正。模型车头为局部 +Z；世界行驶 angle 为 0 时朝 +X，正值朝 +Z 转向，模型 yaw 为 `π/2-angle`。motion 独立使用米/秒/弧度，包含 x/z/vx/vz/angle/yawRate/steering/longAccel/lateralAccel/rpm/gear/shiftTimer/throttle/brake/handbrake/reverseTimer。外部 Car 的 x/y/vx/vy、道路高程/路宽与 progress 仍为分米。motion 的滤波状态与原始输入分开持久化。

protected 是仅用于投影的保护标记，来自已应用保护输入、输入超时或复位期限，客户端不能设置它。保护制动强制油门零、全刹、禁止倒车。有效手动输入可立即进行新的本地预测，远端预测必须保留保护含义。

公开 `horn` 由已应用输入派生，与位置使用同一权威时间线，不能从最新待执行输入提前投影。保护、输入过期、取消托管、合法复位和比赛结束会清除喇叭；false 省略，旧输入和旧存档继续兼容。客户端据此驱动声音播放，不据此判断碰撞或比赛结果。

公开浮点进行小于毫米的位置量化，原始存档保持完整精度；24 条事件历史完整保留，仅量化公开坐标和强度。六车动态 JSON 的固定上限为 8 KiB/消息。

## v3 可撞毁物件

可撞毁白名单仅包括原数据中位置与属性一致的 normal/type 2 对象，France 29、Coastal 83、UK 108、Park 45，共 265 件。其他 type 2、billboard、special、type 1、墙和护栏继续原有碰撞。旧 route、geometryHash 和四份 hash-named 数值规则文件保持原字节；新增数值 catalog 通过 geometry 与原规则文件 SHA-256 绑定原 `Barriers[]` 的完整边索引，不能以空间邻近代替身份。

State 与 View 顶层增加 `props?:{catalog:string,broken:string}`。catalog 为完整 64 位 catalogHash；broken 为标准 base64 字符串，解码长度固定为 `ceil(物件数/8)`，0-based index 使用第 `index>>3` 字节的 `1<<(index&7)` 位。全零位图也公开；UK 最大 108 件，14 个原始字节。catalog、静态道路和空间网格可共享，位图必须按比赛独立持有，不修改共享 `Track.Barriers`。

首次真实接近接触由权威物理步置位，立即跳过该物件全部边的静态墙反弹；同一物件不重复触发，后续车辆和电脑选线不再受其阻挡。高度不相交、静止或远离接触不触发破坏；固定墙保留原有求解。客户端不能通过 drive 或额外动作提交破坏状态。

共用原有 24 条事件环，新增 `type:"prop"` 与可选 `prop:number/dir:number/high:boolean`。prop 为 0-based index，0 必须保留；dir 为碰撞前世界速度的 `atan2(vZ,vX)`，以弧度量化到 0.001；high 在碰撞前法向接近速度不低于 8 m/s 时为 true。8 m/s 是本项目的重建约定，未作为原版实机阈值确认。事件 X/Y 为 0，由 catalog 定位物件；At、Seat 与单调事件 ID 沿用现有时间线。原始 low/high clip 的 6／17 帧、延时 6 和社区 64 Hz 标尺由渲染层解释，不能声称为原版运行测量。

位图是当前世界状态的依据，事件只提供有界的动画信息。迟到观战、丢帧或已超出事件环时，按位图隐藏已破坏物件，不通过重播旧动画重建世界。客户端预测仅复制位图做本次局部接触推算，不得改已确认位图或提前隐藏公共场景物件。

旧 v3 存档缺少 props 时，按原 geometry 固定首版 catalog、默认全部未破坏。没有序列修复的 Restore 保留原存档字节，首次正常 timer 保存才记录明确版本，不重置车况、epoch、检查点、排名或期限。已有明确版本不得静默换成最新 catalog；未知版本、几何不匹配、错误 base64、错误长度或未用高位非零均拒绝恢复。physics=0 不使用这套状态。

实现前失败清单见 [classic-props-failures.md](tests/classic-props-failures.md)，普通 drive 接触与存档 E2E 在 `classic_props_e2e_test.go`，跨语言规则对照在 `classic_props_contract_test.go`。

## v3 驾驶和比赛

3 秒发车、最长 20 分钟、首车后 30 秒、按实际过线时间与合法检查点进度排名的契约保持。使用独立 SI 双轴运动模型，车速由原始传动与阻力决定，不继承旧 90 m/s 统一上限。轮胎、变速、路肩、倒车和设计选择见 [MOTION.md](../../docs/research/racing-nfs/MOTION.md)，未将独立设计结果冒充原版实测。

道路外 3 m 内连续改变抓地和最大阻力；最大阻力在低速 0～6 m/s 连续建立，不在原地超过可用牵引，也不瞬间裁切速度。道路、墙和车的接触按高程分层，防止桥梁与交叉道路的二维误碰。有效门线要求按序正向经过、车身与原路宽或其两侧 3 m 路肩有物理重叠且位于当前道路高度。路肩的计圈范围与驾驶模型一致；复位不增加已完成圈数，也不跳过后续门线。

电脑只发送正常 drive。它们用公开车型能力选择保守行驶包络，以曲率、制动距离和转向建立延迟预看；按原生左右宽度和真实车身胶囊检测前方可行侧，不能假定 VROAD 参考线永远畅通。受阻时正常倒车/前进恢复，禁止改坐标、reset 或注入完赛。测试覆盖真实混合车型，不能用全部 P911 替代五车型验收。

## 版本兼容边界
- 正常新界面仅提供 `france/coastal/uk/park` 四条原版近似路线，新比赛冻结 `physics=3`，使用有独立传动、制动和抓地参数的经典车型。旧 `metro/harbor/woodland/coast/canyon` ID 仅保留显式旧请求和历史房间的 `physics=0` 兼容，不计作新版交付赛道。未带 `physics` 的历史快照不被升级，不替换已保存几何。
- 新版等待准备动作只允许当前席位选择自己的 `vehicle`。比赛中仅保留旧 `skin` 外观兼容，不能改变性能车；六席选项可供较少实际玩家开局，多余车型不进入比赛。
- v3 几何使用完整 64 位 `geometry` 摘要；保存只引用已嵌入的不可变数值规则版本。客户端模型、贴图和音频不嵌入服务器。动态 View 不携带道路、墙体、道具或内部待执行输入。
- 原生道路中心点、高程、法线、真实左右向量和分段路宽全部保留，长度统一转换为分米。道路与墙体碰撞按高度分层，物理内核以 SI 单位推进。道路宽度外 3 m 内连续改变抓地和拖滞；低速拖滞随速度建立，不在原地超过可用牵引力，也不直接截断车速。
- `motion` 保存独立的油门、制动、手刹、转向、变速与倒挡等待状态；`protected` 仅是公开投影中由已应用保护输入、输入过期或复位期限派生的标记。保护全刹不允许倒车，新的有效驾驶输入可恢复普通控制。
- 电脑只使用公开道路、车况与正常 `drive` 输入。按曲率、制动距离和转向建立时间提前控制速度，遇到中心线旁障碍时按真实碰撞和可用路宽选线，受阻时正常倒车恢复；不得复位、传送或写入进度来完赛。

## 旧版兼容契约（physics=0）

本段保留 v2 原始契约，仅适用于显式旧 ID 请求和没有 physics 标记的历史快照；不作为新版交付内容。`racing`，2～6 人，纯竞速；规则、物理、赛道和电脑独立维护。

### 规则与操作

创建选项：`{track:"metro"|"harbor"|"woodland"|"coast"|"canyon", laps:1|3|5}`，历史版本默认霓虹城区、3 圈；当前旧客户端兼容请求应显式指定旧 ID，新界面默认 France。五条长赛道分别提供城区、港湾、林间、海岸与峡谷环境。3 秒起跑灯后同时起步；首次完赛后余车有 30 秒完赛期限；新比赛最长 20 分钟。未完赛按有效圈数和有序检查点进度排序。相同进度为同名次；所有人完全无进度视为和局。车辆外观不改变任何物理参数。

动作均为 JSON；由平台认证当前席位，不接受客户端位置、速度、圈数、时钟或名次：

- `drive {type:"drive", epoch:string, seq:正整数, steer:-1..1, throttle:0..1, brake:boolean, drift:boolean}`：按席位严格递增序号，单次最多跳过 1024、绝对上限为 2³¹−1（保持在 JavaScript 安全整数范围内）；允许其他席位操作产生的旧 revision。方向正数为顺时针／右转。可在倒计时时准备输入。
- `reset {type:"reset", epoch:string}`：只在比赛中、速度不高于 55 世界单位/秒时可用；回到最近已通过的检查点，静止，增加 2 秒重新起步等待，不增加进度；冷却 5 秒。
- `skin {type:"skin", epoch:string, skin:0..5}`：比赛结束前随时可选，六个等性能经典车型；席位颜色与编号独立。

服务器每 100ms 推进一次帧，内部以固定 20ms 积分。最后输入超过 750ms 自动制动；真人空闲不自动托管，公共平台不显示操作倒计时。取消托管只清除本席油门并恢复本席输入期限，不重置赛程或其他席位。

### View

```ts
interface Point {x:number;y:number}
interface RaceView {
 options:{track:string;laps:number};
 phase:'countdown'|'racing'|'finishing'|'finished';
 epoch:string;
 serverAt:number; startAt:number; finishDeadline:number; maxRaceAt:number; // Unix ms
 track:{id:string;name:string;width:number;height:number;roadWidth:number;points:Point[]};
 cars:Array<{
   seat:number;x:number;y:number;vx:number;vy:number;angle:number;speed:number;
   steer:number;throttle:number;brake:boolean;drift:boolean;skin:number;seq:number;
   lap:number;checkpoint:number;progress:number;rank:number;finishAt:number;
   offroad:boolean;lastInputAt:number;idleAt:number;resetUntil:number;
 }>;
 events:Array<{id:number;type:'start'|'lap'|'finish'|'collision'|'reset';at:number;x:number;y:number;seat:number;intensity:number}>;
 controls:{canDrive:boolean;canReset:boolean;canSkin:boolean};
}
```

`angle` 使用弧度，0 向右，y 向下；`speed` 为世界单位／秒。`lap` 是已完成圈数，显示当前圈用 `min(lap+1, options.laps)`。`checkpoint` 是下一待通过的中心线采样点索引；所有点须有序经过其门线且沿正向行驶才增加圈数。`progress` 是有效完成的总路线距离（世界单位），可用于名次和缩略图。`rank` 从 1 开始。`finishAt=0` 表示未过最终终点；`finishDeadline=0` 表示首车尚未完赛。`serverAt` 为物理状态的模拟时刻；倒计时用 `startAt`，余车限时用 `finishDeadline`。`events` 保留最近 24 条、ID 单调递增，客户端只播放新事件，重连不补播。

`track.points` 为实际闭环道路中心线的平滑等距采样，不重复首点；新地图约每 75 世界单位采样并保留一位小数。三维道路以同一中心线与 `roadWidth` 构建路面，与服务器道路检测一致。新增 `track.props?: {x,y,radius,height,kind}[]`，是公开路边实体；地面圆形近似与车辆胶囊接触，位置及外观参数由服务器提供。旧存档中的完整 Track 及已有 `maxRaceAt` 不因恢复而被新目录替换。世界矩形由 width/height 定义；路外阻力更大，世界边缘为软碰撞边界。车辆物理胶囊半径 13、轴向中心线长 20，匹配车身宽 26/长 46，手机可通过镜头缩放保证可辨认；车型共用物理尺寸。

平台保留公共创建、等待房间、玩家栏与托管入口。模块公共玩家统计可用当前 `rank`、圈数、是否完赛。观战 View 与真人同样公开车位，只令 controls 全 false。

### 验收失败场景（先于实现）

- 非法人数、未知赛道、圈数或字段、超范围输入、观察者行动、重复 seq、旧 epoch 被误接受。
- 输入直接改坐标、倒计时提前移动、丢包仍持续油门、计时器重复消费、恢复丢圈数或改期限。
- 逆穿终点、偏离道路抄近路、瞬移或无序经过检查点误刷圈。
- 草地没有减速、漂移不产生侧滑、碰撞非有限数、不同 skin 参数不同。
- 五赛道任一人数（2～6）或圈数（1/3/5）电脑卡住，靠超时或重置判胜；电脑提交位置。
- 真人/电脑混合、托管/接管重置其他席位输入、结束后继续行动、完赛排名不按过线时间。
- 没有完赛期限或最长时限，所有人无进度仍任意指定赢家，快照不可重放、竞态数据污染。
- 比赛中换车身被拒绝，或换车身影响输入序号、进度、物理、任何席位期限。

测试将经生产 Module 的 Start→BotAction/Apply→OnTimer→View→Result 完整比赛，加 HTTP 平台入口验收；保存可重跑结果到 `.local/racing`。

### 客户端短预测物理（不判定圈数、名次或碰撞结果）

固定步长 0.02 秒；道路最大速 900 世界单位／秒（324 km/h）；油门加速度 245。转向角速度为 `steer*1.65*min(abs(forward)/65,1)/(1+(abs(forward)/220)^2)`，漂移时乘 1.18。先推进车身角，再将原 `vx/vy` 投影到新车身方向，施加油门和前向阻力 `exp(-.2*dt)`；手刹额外乘 `exp(-1.2*dt)`，刹车额外乘 `exp(-6*dt)`；刹车优先，按下时不再施加油门推力。侧向速度乘 `exp(-9*dt)`，漂移时为 `exp(-1.35*dt)`；重组世界速度后积分位置。路外前向阻力为 `exp(-2.5*dt)`、侧向阻力为 `exp(-4*dt)`、最高速 110。单次输入生效时间按服务器接收时刻排队，不能用较晚输入追溯修改已流逝的物理时段。客户端只预测自身近端位移，随后按 seq 和权威位置平滑校准。

### 异常旧序号恢复

合法输入不能从小序号跃升到整数边界。电脑遇到越界／耗尽的旧序号时先返回空行动，确保本帧计时器仍能推进；恢复存档或下一帧计时器只清除异常席位的输入队列和控制量，将其 seq 归零，并更换本场 epoch 拒绝旧请求。位置、速度、圈数、得分、其他席位输入及所有既有期限保持不变。正常存档不改变 epoch 或任何内容。该异常修复只防御未发布旧存档或异常数据，不是比赛中的主动重开操作。

### 实时状态保存

普通输入与物理帧在内存中推进，不逐条写动作日志或房间快照。平台每秒保存发生变化的房间；开局、结算、托管切换、关闭及 `CheckpointKey` 标记的关键阶段立即事务保存。正常断线重连读取内存最新状态，正常服务关闭保存剩余变化；异常崩溃在存储健康时可能回退约一个保存周期，保存失败时按最后成功检查点恢复。

输入确认的 `durable: false` 表示服务器已处理，不能作为落盘凭证。新实例通过 `serverInstance` 通知客户端接受恢复版本，拒绝旧实例控制，并清理手动席位的旧方向／按键。电脑及手动托管保留；比赛身份、结果与系统期限仍由服务器维护。

### 追车画面和额外操作

主视角为车后透视，B 为本地按住回看，不生成游戏行动；按键与触屏来源独立管理。松手、取消、失焦、重连与换局清理旧控制，过期键盘重复不自动恢复。空格对应协议既有 drift 字段，表示手刹；R 复用服务器 reset 规则。自动挡按速度显示，不接受手动 gear 行动。

三维物体和六款车型由项目生成，场景摄像机、车轮与材质只负责呈现。减少动态效果关闭速度引起的镜头变化和装饰轮转；必要的车辆运动仍按服务器时间呈现。图形上下文丢失使用透视兼容画面。离桌清理 RAF、观察器、WebGL 资源和平台循环音效。

### 车型扩展

0 保时捷 911 Turbo；1 兰博基尼 Countach；2 法拉利 F40；3 日产 GT-R R34；4 丰田 Supra A80；5 宝马 M3 E46。外观网格为风格化程序模型，不是原游戏资源或汽车扫描件。所有车型共用规则参数、车身碰撞范围和操作接口；增加车型时须同时更新目录、合法 ID、外观及等性能验收。
