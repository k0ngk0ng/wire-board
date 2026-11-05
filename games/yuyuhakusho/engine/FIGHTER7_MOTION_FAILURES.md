# 来源 7「树」MAIN 失败场景

本清单先于 `fighter7_motion.go` 和测试实现保存。范围止于动作分派入口到共享动作尾 `15afc`；原版 CPU 仅用于采集证据，生产实现独立运行。

## 来源与边界

- 固定 ROM SHA-256：`c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06`。
- 原版控制、合法菜单输入、2400 帧来源 7 初态与 3602 帧无钩子参考由主任务统一生成；本范围不能重建另一条控制链。
- MAIN：`39/1bd6e`、`3a/1be08`、`3b/1be64`、`3c/1bfce`、`3d/1c032`；共享支持：`0f/1ca9c`、`11/1cb08`、`13/1cbc8`、`14/1cc28`、`45/1c6e2`。
- 使用现有 `ChuSpecialMotionState` 的完整 Energy、AlternateVelocity、Body 与 Children；`AlternateVelocity` 是 `+48` 唯一临时视图。`Left` 对应 `a010`，ChildExtras 复用 `Fighter4ChildExtra` 中现有 Held／Pressed 字节。物理 `+fe` 保持 DragonBank 唯一 owner。
- OtherFighters 恰好提供另外三个固定父体 `8400/8800/8c00/9000`；自身 `+69` 留在既有 actor owner，不在 map 复制。MAIN 只改 map 中的 HitStopTicks；调用者负责一次合并。

## 必须失败的场景

1. `39` 再按 Facing 镜像已带符号的 VX；先更新 Y 再更新 VY；有符号 long 溢出被饱和；仍上升时错误触发落地或碰撞；ContactFlags 位 2／3 没有置 `+48` 最高 byte 的 bit0；将该标记写到 long 的低 bit；仅比较完整 Y 导致小数部分影响 word 落地；ground clamp 丢失小数；落地不按旧标记选择 `a1/a2`；非零 frame 或无 contact0 时错误跳帧；跳帧没有清 Ticks；完成标记被无条件消费。
2. `3a` 完成值不是精确 1 仍瞬移；将 `+48` 低 byte 误作变体；变体 0／1／2 与其他值的 Left 偏移错误；word 加法没有回绕；改写 X 的小数低 word；没有同步 Ground→Y；遗漏 Request／EnabledStages 清零或 reset；错误清除运动字段。
3. `3b` 完成值不是精确 1 仍转入 `3c/a7`；只尝试一个子槽；子一忙时跳过空子二；子二忙时覆盖旧值；Attributes 使用固定 OR2000 而非 OR6000；父 Attributes 加偏移／mask 的顺序不符；父 Team reset 前后的取值不符；TargetBit 的 byte 自增无回绕；Y 只改高 word 的 -32 写错为完整 long；X 偏移不按 Facing 非零分支；错误镜像小数／VX；错误复制整个 Ground long；Layer 的 +/-4 无 byte 回绕；遗漏 TargetMask=ffffffff；覆盖未被原版写入的速度、Power、Pose、输入、FE 或其他 dirty bytes。
4. `3c` 仅扣 Energy 高 word；遗漏低 word 借位；把零值当负值；负值未清完整 long；long 减法回绕后的符号判定不符；能量不足仍复制子输入；子一忙闲判断前未复制子一 Held／Pressed 低 byte；子一空仍复制子二；子二空被错误过滤；复制父控制扩展字节或 Changed；C 退出时序先于必要输入复制；退出未进入 `03/a8`、清 completed 并 reset；非退出路径修改动画／速度。
5. `3d` completed=1 未优先返回站立；其他 completed 值被当作完成；不是 frame2/ticks0 时创建子三或写停顿；子三忙时未继续写其他父体；初始化子三时写了原版未触及字段；按 Size／Health 排除 empty/dead 父体；按 BaseTeam 或角色来源判断；遗漏某个固定父体；误写 StunTicks(+84)；改写同队或自身；重复持久保存 HitStopTicks。
6. `0f/11/13/14/45` 仅引用其他来源测试，未以合法 source7 动画／运动参数验证；shared Completed 的精确比较、Facing bit／非零差异、word 落地比较、碰撞优先级、速度衰减、dust/trail 分配或空池行为有偏差；退步表索引越界不返回错误。
7. 多步轨迹每帧重建动作初态或注入期望输出；用 Go 自造时钟替代原版 `19c96`；未依据自身当前不可变动画 key 刷新 Duration／Flags；漏掉跨 `39→03`、`3a→03`、`3b→3c→03`、`3d→00` 和共享动作退出的真实里程碑。
8. 完整 64 KiB 差异未检查；将 `7000..77ff` 当作无条件容许 history；将外部音频队列清除或 IRQ RAM 改写误归 MAIN；吞掉 RNG 改变；漏保留逐链 before/after、调用栈和 core/ROM/input/脚本/压缩夹具摘要；钩子运行与唯一无钩子 RAM／video 参考不一致。
9. 不支持动作、无效 ObjectID、缺少／多余 peer、无效 FX cursor 失败时没有返回原始 state；错误返回泄漏 map 部分改写或修改调用者 map；成功调用原地修改输入 map。map 中任何 peer 缺失必须错误并原子回滚，不能静默跳过。

## 先行验证顺序

先采自然 MAIN 及共享支持调用、dirty 边界和原 CPU clock 的单初态轨迹，再冻结测试源码及夹具摘要，实际运行得到本范围 `StepFighter7Motion`／`Fighter7MotionState` 缺失的 red。实现后运行完全相同的断言，保留 native 与 WebAssembly 结果。不得为通过实现而调整冻结断言；发现采集器错误应回到证据阶段并明确记录新的冻结版本与原因。
