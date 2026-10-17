# 原版队友支撑接近策略 mode29

## 实现前失败场景

- 全电脑cold519的mode28恢复后，同阶段下一席位进入配对mode29。原AA2C只决定接近、跳跃或攻击动作，不能在AI内强行依附队友。
- 首次进入设置TrackedObject=actor^1、Destination=4；持续调用保留原缓存。不重置其他计划或随机流。
- Action3D直接等待；Action4根据原JumpGrade、JumpPhase与无符号速度字节决定二次跳跃或朝向持球对象。
- 普通接近用ADA4的严格X<9、非对称Depth[-4,8)窗口；X远时原固定斜向下跑，不替换为最短路。未到达、未腾空且depth区块>=5时原覆盖为向下走。
- 到达只发StopPlanar、HeadingNone、Jump动作；保持纵向速度和其他状态由后续物理处理。
- Action3F用B05C的X<16、Depth绝对差<8判断持球对象；满足后才请求攻击65并mode0。保持字节回绕、方向和Facing的原先后顺序。
- 原函数隔离矩阵与自然调用分开记录；不将未恢复的物理动作变成假成功。

## 原函数证据与验证

独立REA证据`ev_0513f7e82b4cc05cb6d447b7ebccc67037a0580eeb3c5d263c8272ae8aa6f29a`位于`.local/basketball/rea/ai-rider-approach/analysis/`，覆盖AA2C、ADA4、B05C、B1E9。保留原窄X窗口中的非对称深度窗口、X远时固定向下斜跑、跳跃阶段的无符号速度门槛，以及3F姿态下持球目标接近窗口。

原18,000帧自然参考未命中AA2C；零命中记录保留，不能冒充自然验证。原AA2C连同其完整辅助函数另执行9176组隔离场景，覆盖接近、等待、跳跃、grade、phase、速度、持球位置与字节边界；比较全部Go决策与实际应用后的平面速度／JumpPhase，结果相同。

AI只通过既有StopPlanar及JumpPhase命令请求原动作，无新的世界状态或配置字段。原生竞态／Wasm通过；全电脑cold随后两次6000更新一致。物理或道具动作没有在本文件绕过或补写。

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_ai_rider_approach.py
source scripts/env.sh
go test ./games/basketball/engine -run TestAIRiderApproach -count=1
```
