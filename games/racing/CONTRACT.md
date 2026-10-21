# 极品飞车模块契约 v2

本文件先于实现建立。`racing`，2～6 人，纯竞速；规则、物理、赛道和电脑独立维护。

## 规则与操作

创建选项：`{track:"metro"|"harbor"|"woodland"|"coast"|"canyon", laps:1|3|5}`，默认霓虹城区、3 圈。五条长赛道分别提供城区、港湾、林间、海岸与峡谷环境。3 秒起跑灯后同时起步；首次完赛后余车有 30 秒完赛期限；新比赛最长 20 分钟。未完赛按有效圈数和有序检查点进度排序。相同进度为同名次；所有人完全无进度视为和局。车辆外观不改变任何物理参数。

动作均为 JSON；由平台认证当前席位，不接受客户端位置、速度、圈数、时钟或名次：

- `drive {type:"drive", epoch:string, seq:正整数, steer:-1..1, throttle:0..1, brake:boolean, drift:boolean}`：按席位严格递增序号，单次最多跳过 1024、绝对上限为 2³¹−1（保持在 JavaScript 安全整数范围内）；允许其他席位操作产生的旧 revision。方向正数为顺时针／右转。可在倒计时时准备输入。
- `reset {type:"reset", epoch:string}`：只在比赛中、速度不高于 55 世界单位/秒时可用；回到最近已通过的检查点，静止，增加 2 秒重新起步等待，不增加进度；冷却 5 秒。
- `skin {type:"skin", epoch:string, skin:0..5}`：比赛结束前随时可选，六个等性能经典车型；席位颜色与编号独立。

服务器每 100ms 推进一次帧，内部以固定 20ms 积分。最后输入超过 750ms 自动制动；真人空闲不自动托管，公共平台不显示操作倒计时。取消托管只清除本席油门并恢复本席输入期限，不重置赛程或其他席位。

## View

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

## 验收失败场景（先于实现）

- 非法人数、未知赛道、圈数或字段、超范围输入、观察者行动、重复 seq、旧 epoch 被误接受。
- 输入直接改坐标、倒计时提前移动、丢包仍持续油门、计时器重复消费、恢复丢圈数或改期限。
- 逆穿终点、偏离道路抄近路、瞬移或无序经过检查点误刷圈。
- 草地没有减速、漂移不产生侧滑、碰撞非有限数、不同 skin 参数不同。
- 五赛道任一人数（2～6）或圈数（1/3/5）电脑卡住，靠超时或重置判胜；电脑提交位置。
- 真人/电脑混合、托管/接管重置其他席位输入、结束后继续行动、完赛排名不按过线时间。
- 没有完赛期限或最长时限，所有人无进度仍任意指定赢家，快照不可重放、竞态数据污染。
- 比赛中换车身被拒绝，或换车身影响输入序号、进度、物理、任何席位期限。

测试将经生产 Module 的 Start→BotAction/Apply→OnTimer→View→Result 完整比赛，加 HTTP 平台入口验收；保存可重跑结果到 `.local/racing`。

## 客户端短预测物理（不判定圈数、名次或碰撞结果）

固定步长 0.02 秒；道路最大速 900 世界单位／秒（324 km/h）；油门加速度 245。转向角速度为 `steer*(1.65+.002*abs(forward))*min(abs(forward)/65,1)`，漂移时乘 1.18。先推进车身角，再将原 `vx/vy` 投影到新车身方向，施加油门和前向阻力 `exp(-.2*dt)`；手刹额外乘 `exp(-1.2*dt)`，刹车额外乘 `exp(-6*dt)`；刹车优先，按下时不再施加油门推力。侧向速度乘 `exp(-9*dt)`，漂移时为 `exp(-1.35*dt)`；重组世界速度后积分位置。路外前向阻力为 `exp(-2.5*dt)`、侧向阻力为 `exp(-4*dt)`、最高速 110。单次输入生效时间按服务器接收时刻排队，不能用较晚输入追溯修改已流逝的物理时段。客户端只预测自身近端位移，随后按 seq 和权威位置平滑校准。

## 异常旧序号恢复

合法输入不能从小序号跃升到整数边界。电脑遇到越界／耗尽的旧序号时先返回空行动，确保本帧计时器仍能推进；恢复存档或下一帧计时器只清除异常席位的输入队列和控制量，将其 seq 归零，并更换本场 epoch 拒绝旧请求。位置、速度、圈数、得分、其他席位输入及所有既有期限保持不变。正常存档不改变 epoch 或任何内容。该异常修复只防御未发布旧存档或异常数据，不是比赛中的主动重开操作。

## 实时状态保存

普通输入与物理帧在内存中推进，不逐条写动作日志或房间快照。平台每秒保存发生变化的房间；开局、结算、托管切换、关闭及 `CheckpointKey` 标记的关键阶段立即事务保存。正常断线重连读取内存最新状态，正常服务关闭保存剩余变化；异常崩溃在存储健康时可能回退约一个保存周期，保存失败时按最后成功检查点恢复。

输入确认的 `durable: false` 表示服务器已处理，不能作为落盘凭证。新实例通过 `serverInstance` 通知客户端接受恢复版本，拒绝旧实例控制，并清理手动席位的旧方向／按键。电脑及手动托管保留；比赛身份、结果与系统期限仍由服务器维护。

## 追车画面和额外操作

主视角为车后透视，B 为本地按住回看，不生成游戏行动；按键与触屏来源独立管理。松手、取消、失焦、重连与换局清理旧控制，过期键盘重复不自动恢复。空格对应协议既有 drift 字段，表示手刹；R 复用服务器 reset 规则。自动挡按速度显示，不接受手动 gear 行动。

三维物体和六款车型由项目生成，场景摄像机、车轮与材质只负责呈现。减少动态效果关闭速度引起的镜头变化和装饰轮转；必要的车辆运动仍按服务器时间呈现。图形上下文丢失使用透视兼容画面。离桌清理 RAF、观察器、WebGL 资源和平台循环音效。

## 车型扩展

0 保时捷 911 Turbo；1 兰博基尼 Countach；2 法拉利 F40；3 日产 GT-R R34；4 丰田 Supra A80；5 宝马 M3 E46。外观网格为风格化程序模型，不是原游戏资源或汽车扫描件。所有车型共用规则参数、车身碰撞范围和操作接口；增加车型时须同时更新目录、合法 ID、外观及等性能验收。
