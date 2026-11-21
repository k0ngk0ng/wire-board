# CS 1.6 规则核心

本目录实现服务器权威规则，并向浏览器 WASM 提供同源运动预测。世界坐标保持 GoldSrc 的 X/Y 水平、Z 向上；人物位置是碰撞盒中心，人质位置是脚底。

## 数值契约

| 数据 | 单位 |
| --- | --- |
| 位置、速度 | 原 GoldSrc 单位乘 256；速度按秒 |
| 方向 | 一圈 65536；正俯仰朝下 |
| 法线、射线比例 | 1048576 为 1 |
| 规则时钟 | 100 Hz，每步 10 ms |
| 枪械周期 | 微秒累计，映射到规则步执行 |
| 门运动进度 | 0 至 1000 |

站立人物 hull 为原地图 hull 1，蹲下为 hull 3。站立眼高是中心上方 17 个原单位，蹲下是 12，依据所选 CS 源码中的 `PM_VEC_VIEW`，不是 Half-Life 的 28。跳跃初速由整数平方根计算 `sqrt(2 × 800 × 45)`。业务计算没有浮点阈值。

四张原图的平面、clipnodes、brush 模型、出生点、目标区和原 PODBot 导航由 `scripts/cs16/build_assets.py` 导出。射线使用 hull 0；人物运动使用对应的扩张 hull。动态门的位移与旋转参与相同 trace，旋转门按触发者方位远离玩家开启；活人使用连续扫掠阻挡，死亡玩家不阻挡。梯子、水中运动、台阶和滑墙均由同一运动代码处理。

`PredictMotionWithDynamics` 接收本人、可见邻近玩家和动态门状态，仅返回预测本人。合法切枪可以立即更新本人显示；生命、伤害、购买、命中与比赛结算保持服务器权威。碰撞、枪械与目标规则不会接受客户端上传的最终结果。

## 输入与恢复

输入分别维护接收序号与已应用序号，离散动作边沿按规则步消费。排队超过 20 帧的输入不补播；30 帧没有有效输入会释放瞬时操作。输入队列、事件与历史均有明确容量。清理输入也取消尚未主动投出的手雷。

冻结准备期允许库存切换、装填与武器模式操作，攻击、投掷和伤害仅在实战阶段生效。烟雾遮挡与浏览器共用中心、半径及成长期限，电脑感知和真人敌方投影均按整数线段与球体求交，不穿烟追踪。

`Options.FriendlyFire` 默认关闭并在开局固定。开启后采用所选原规则的枪／刀 35% 队伤与 HE 队伤；队杀扣 1 frag 和 3300 资金，最低为 0。队友始终参与真实碰撞、命中与穿透，关闭队伤只阻止队友生命和护甲损失。自身伤害和 C4 目标爆炸不随开关改变。电脑保持敌人目标选择，并在每次输入输出时检查原 MDL 身体射线，队友挡在首个命中位置时停止枪／刀攻击。

`Advance` 每次最多推进 1000 步，重复或较旧目标帧不推进。服务器在长停顿时通过后续有界回调追赶，不在单个回调内同步执行整场比赛。恢复权威状态时须重新绑定 `World=MapByID(MapID)`，不将地图常量重复存入每帧状态。

导航距离场与 face 平面索引在服务器首次读取地图时构建，然后保持只读。地图与模型规则按需初始化；纯运动 WASM 不需要解析全部四张地图或战斗命中姿态数据。

## 原版依据与范围

参数参考固定 ReGameDLL_CS 提交 `1f16a7f25c14225931f8df1abc51f68d18ded839`，选择未开启 `REGAMEDLL_FIXES` 的原规则分支。枪械来源、逐枪参数和分支差异见 [WEAPON_REFERENCE.md](WEAPON_REFERENCE.md)。地图和模型属于第三方归档样本，来源身份和摘要由研究清单保留，不将公开参考实现当作原版运行证据。

当前规则是在定点整数和固定步长下重建。BSP 平面量化、源浮点舍入、用户命令持续时间及原版可变帧率之间仍可能存在轨迹差异。命中盒采用原 MDL 的角色、朝向、武器与站蹲姿态；动画基准姿态通过独立数据对照，不宣称全部动作逐帧等价。网页性能、网络延迟和原版运行一致性需要各自的验收，不能用规则 E2E 代替。

## 验收入口

失败场景先于实现，见 [FAILURES.md](FAILURES.md)、[COMBAT_FAILURES.md](COMBAT_FAILURES.md) 与 [OBJECTIVE_FAILURES.md](OBJECTIVE_FAILURES.md)。规则回归包含原图 10 席完整比赛、33 Hz 输入与 100 Hz 推进组合、原图目标流程，以及全部 24 种枪械的实际购买、开火、装填和特殊模式。

```sh
mkdir -p .local/cs16/go-cache .local/cs16/go-mod .local/cs16/tmp .local/cs16/verification
GOCACHE="$PWD/.local/cs16/go-cache" GOMODCACHE="$PWD/.local/cs16/go-mod" GOTMPDIR="$PWD/.local/cs16/tmp" TMPDIR="$PWD/.local/cs16/tmp" GOTOOLCHAIN=local GOTELEMETRY=off go test -json -count=1 ./games/cs16/engine > .local/cs16/verification/engine-full.jsonl
```

`engine-red.jsonl` 保留实现前的失败记录；`engine-motion.jsonl` 记录运动、输入与预测对照。最终完整结果以 `engine-full.jsonl` 的实际状态为准，浏览器与生产模块结果由对应验收产物另行提供。
