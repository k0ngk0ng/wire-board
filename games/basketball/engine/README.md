# 篮球逻辑内核

状态：已恢复三轴运动、动画、基本无球／持球动作、普通投篮及篮筐对象、接触／接球组件，以及部分原版电脑策略。完整战斗、特殊动作、比赛循环与浏览器接入仍未完成。

`Position` 区分球场横向 X、球场纵深 Depth 和离地 Height。`Velocity` 使用同样的三个方向，`Gravity` 控制每次模拟更新的竖直加速度。这些都是有明确游戏含义的数据，不是模拟器寄存器或内存地址。

`Fixed` 每像素使用 256 个单位，以保留原版小数积累和进位。`IntegrateMotion` 先更新竖直速度，再计算位置；后续碰撞阶段负责着地、场地边界及其他接触。当前函数故意不自行增加地面钳制，因为原版在另一个阶段处理这些规则。

原版观察数据、来源摘要和字段说明见 [testdata](testdata/README.md)。

`AdvanceAnimation` 根据明确的帧、停留时间和循环方式推进姿态；动画完成后请求切换动作。`StepFreePlayer` 处理待机、行走、跑动、急停、普通／跑动跳跃、落地、A／B 出招，使用 `CharacterProfile` 中可修改的速度、重力和动画数据。攻击命中及伤害尚未实现。

当前场地处理仅适用于纽约场地的平地区域。进入尚未恢复的平台／障碍或未实现动作时会明确返回错误，不以穿墙、静止或强制待机掩盖缺失。原版纽约场地的内部编号为 1。

起跳初始化在原版同一世界更新中会再处理一次角色；Go 明确保留这个第二阶段。落地修正会恢复上一时刻的地面位置，再进入落地动作，避免跑动跳跃多滑一段。左右急停的减速量也按原版分别保留，未擅自改成对称值。

独立的 `cmd/verify-player` 连续推进整段状态，每步只补入原版记录的动作请求与方向，不重置 Go 位置、速度或动画。11 组场景、633 次完整动作阶段均与原版一致；原生及 Wasm 的连续状态 SHA-256 相同。这不等于浏览器输入或完整对局通过。

```sh
source scripts/env.sh
go test -count=1 ./games/basketball/engine
go run ./games/basketball/cmd/verify-player

# 验证同一 Go 代码编译为 Wasm 后的数值结果。
mkdir -p .local/basketball/engine
GOOS=js GOARCH=wasm go test -c \
  -o .local/basketball/engine/motion.test.wasm ./games/basketball/engine
basketball_goroot=$(go env GOROOT)
(
  cd games/basketball/engine
  node "$basketball_goroot/lib/wasm/wasm_exec_node.js" \
    ../../../.local/basketball/engine/motion.test.wasm -test.v
)
```

该 Wasm 检查在 Node 中运行，不代表浏览器呈现或联机输入已经接入。最终 Canvas 只读取内核状态绘制，比赛结果由服务端同一份 Go 规则确认。
