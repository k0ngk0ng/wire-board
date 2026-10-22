# 疯狂坦克副弹、坡度与时限复核

本次对照当前仓库与保留的官方网页客户端，未运行原客户端或取得原服务端。复核使用 REA、WABT 1.0.42；原始 WASM SHA-256 为 `35974c0d7a06cf86f1f028b0ee1fc46c324921f6d829ce4db684f776199e489d`。

## 副弹和追踪

14 车型已具有独立副弹行为；本次逐项复核[副弹分派表](../fortress-secondary/README.md)，没有把已经存在的散射、地雷、治疗再次描述为本次新增。重新核对 55 个原函数字节范围及哈希，并通过 REA 重新解码同一切片，Evidence ID 保持 `ev_aa61593bdab830e5487eb117817a4526e22dd84743146acd31ae1df7406931c0`。

飞火和暗影的副弹分别通过 `f2992`、`f3154` 调用 `f1774`，下降接近目标后锁定一次位置并改向，不是无限持续追踪。当前实现保留这一行为。原函数遍历角色注册表且不排除友军，不能擅自改成只追敌人。150 世界像素搜索窗和速度系数仍为平台参数，原配置值未恢复。

真实浏览器使用生产服务器和原生空格按下／释放，覆盖 14 车型的主副弹共 28 组，检查弹数、独立图片帧、逐弹出膛／命中、地雷和延迟离子束。创建弹窗同时确认默认 60 秒。完整结果为 `.local/fortress-oct23/browser-projectiles-final/report.json`。另外对 6／9／10 号进行低力锁定、地雷和延迟离子束的 6 组重点验收，实际画布锁定标记、地雷图片及延迟光束均通过，见 `.local/fortress-oct23/browser-effects/report.json`。

## 坡度修正

此前车身使用坡度图片，但发射方向仍直接采用输入角度。原客户端 `f3303` 在地面高度采样后计算倾角，并将该倾角加入炮角字段 `+196`，左右方向另行转换；因此只改变图片是不完整的。

现在服务端以实际地形和弹坑计算附近左右支撑，将车身相对炮角转为世界方向。电脑模拟使用同一函数；前端炮口指示线、角度盘指针及射界边界同步，点击世界位置先扣除坡度再写入相对炮角。没有附近成对支撑时保持水平参考，避免把远处平台当作履带。已发射的轨迹存档继续使用原路径。

先行测试复现了坡地仍以 45° 发射的问题。真实 HTTP 及 SQLite 验收使用 `stage-16` 的实际斜面，45° 相对炮角朝左／右的初始路径分别为 63.378°／26.451°，与连续地形预期 63.435°／26.565° 一致（差值来自首积分步重力）。恢复后原轨迹完全一致。证据：`.local/fortress-oct23/slope-http.log`。

最新构建的 PC／320px 真浏览器验收还验证：原生移动从斜面驶到平地，相对炮角保持 45°，世界角度盘从 36° 变为 45°；反转朝向后坡度符号正确，指针坐标与实际地形一致。记录和截图见 `.local/fortress-oct23/browser-slope-final/report.json`。

## 等待时间

服务端默认值、创建与等待房间共用的规则表单默认值均为 60 秒，选项保留旧 15／20／25／30／40 秒并增加 60 秒。已有对局的自定义时间和正在执行的期限不重置。选车阶段仍为 20 秒；这是开赛选择阶段，不是行动思考时间。

## 风向更新时间

原版在每位玩家开始行动的消息中接收风值，不是必须等所有玩家行动完一轮。调用链已从网络分派闭合到当前玩家切换：

1. `f2411` 的消息 `7310` 分支打印原始字符串 `S_PLAY_TURN`（内存地址 `49726`），再调用 `f1496`。另一个分派入口 `f2357` 打印 `S2C_PLAY_TURNSTART`（地址 `47472`）后同样调用它。
2. `f1496`（日志 `OnGetSPlayTurnMsg`）读取包首 4 字节回合计数、`+5` 的有符号风值，调用 `f337` 写入全局风状态 `+2920`。该写入没有“风必须与上一回合不同”的检查。
3. 同一消息 `+32` 的玩家部分传入 `f1548`；后者读取当前行动玩家，打印 `NowPlayer %d`，更新当前玩家字段并推进角色行动状态。天气事件可能先插入过场，但不是全员轮次计数后才接收风。

当前平台的主要边界一致：`advanceTurn` 在弹道及落地结束后、或者行动超时交给下一席时更新风；同一席移动、调角度或普通开火不会重抽风，飞行中的轨迹也不重算。`windFor(seed, turn, sudden)` 没有强制反向或去重，相邻回合允许同向或同值。风向转换道具可主动反向；平台决胜阶段进入时另有一次增强风力，属于既有平台规则。

因此本次不擅自改成“每两回合”或“全员一轮”才换风。**更新时间核查完成；原服务端选风概率、连续多个回合恰好相同的频率仍未知。** 客户端每回合收到风值，不等于服务器每回合必须抽到不同风。

六函数切片 REA Evidence ID：`ev_8123f32d7779e68f2eb456f68c4b3396d3a08b6d2f39e6a3adaafacce79bce52`。

源字节范围、原字符串偏移和函数页保存在 `.local/fortress-oct23/turn-wind-evidence.json` 及同目录 `f1496/f1548/f2357/f2411.wat`。`scripts/fortressblue/inspect-turn-wind.py` 可从同一原始 WASM 重建六函数切片并核对字符串；未执行原客户端。此前天气 26 函数切片也已重新验证。

已保留的 Steam 官方公告只说明风影响瞄准；官方宣传片存在剪辑，不能用于统计连续回合概率。Google 新查询进入验证码，未尝试绕过；Bing 返回内容与查询无关，未采用其结果。

## 复跑

```sh
. scripts/env.sh
go test -race -count=1 ./games/fortressblue/server ./games/fortressblue/tests
go test -count=1 -v ./games/fortressblue/tests -run '^TestSlopeLaunchAndMinuteDefaultThroughHTTP$'
node --experimental-strip-types --test games/fortressblue/tests/*.test.mjs
python3 -B docs/research/fortress-weather/verify-evidence.py
python3 -B scripts/fortressblue/inspect-turn-wind.py
python3 -B scripts/fortressblue/check-refactor.py --only projectiles --out .local/fortress-oct23/recheck-projectiles
python3 -B scripts/fortressblue/check-refactor.py --only slope --out .local/fortress-oct23/recheck-slope
```

浏览器命令需先有当前源码构建的 `web/dist`，输出目录须为新路径。验收脚本让 HTTP 对手通过实际加载协议确认就绪，真人浏览器仍须完成真实素材解码；没有移除生产加载门闩。

## 清理

已清理本任务独立浏览器 profile／socket、失败的首轮浏览器目录与日志、完整 WASM 解码中间文件及重复 REA 大输出。保留成功报告、截图、输入、原始来源哈希、聚焦函数页和新风时机 REA 记录；清单在 `.local/fortress-oct23/cleanup.json`。未删除共享构建、其他任务缓存、原素材或 OSS 对象。
