# 来源 2「桑原」下一阶段范围

本页保留实现前的静态范围。桑原完整规则、世界组合和五角色对局已按此范围验证，结果及剩余边界见[桑原世界验收](../../../games/yuyuhakusho/engine/FIGHTER2_WORLD_VERIFICATION.md)。

姓名由[原 HUD 字形](CHARACTER_IDS.md)及[真实菜单发现](source2-next-selection.png)确认；[输入](source2-next-inputs.json)与[记录](source2-next-selection.json)仅是原 core 1,800 步发现，双 core 基线、原 CPU 返回及独立规则尚未完成。下列是有限静态观察与当前恢复代码对照，不冒充执行证据。

| 并行范围 | 原入口 | 最小规则边界 |
| --- | --- | --- |
| 四 selector | `16714/16856/168cc/169b0` | 完整 RTS/SP+4；12 特殊构造 `18cc0/18cea/18d14/18de0/18e00/18d3e/18d56/18d6e/18d9e/18db6/18e60/18e36` |
| 三持续槽 | `1ac58/1ad28/1adc8` 对应 `1e/1f/20`，止 `15afc` | 复用已恢复0f/11，负责建立子对象而不推进 |
| 新子对象 | `12dd4`，init12dda、continue12e54 | Draw12ea4→1580，clear12ee2→3556；无父体依赖 |

建议 selector 复用 `ChuAttackState`，motion 复用 `ChuSpecialMotionState`，projectile 在 `ChuProjectileState` 外增加 `HitStopTicks uint8`视图。canonical `WorldObject.HitStopTicks` 已存在，无需新存储。已恢复的散射弹 `12c96` 仍需来源2的初始化／连续上下文；不能复制整个生命周期。

请求1方向表 `1690c..16921`只有11个结构项：0/1/3/7→18cc0、4/5/8/9→18cea、2/6/10→18d14。索引11..15读到相邻指令字，静态计算分别为1c922／18cc0／1c926／18cea／1c92a；其实际 CPU 返回未知。下一阶段将正常有界矩阵与独立诊断分开，不补默认动作、不把“不支持”称原版 no-op，也不一起拒绝其他模式16项表。

实现前失败场景和原 CPU 矩阵必须覆盖：

- 地面请求1不要求 Changed，成本12、方向选择普通03/a0..a2；空中仅请求1直接成本12进02/a3，不能添加方向／Changed／子槽门槛。
- 地面请求2无动作；3主槽空、B优先A、成本18进20/a8，横速完整写40000／80000，仅清Gravity高word；4主槽空、C/B/A优先、成本20进1e/a6，横速高word偏移30／70／b0（这些参数为十六进制）。
- 地面5成本16进1f/a4或a5；6生命≤111、A Changed、成本200进入普通03/aa。普通投技Y−24、宽1a/高24、目标40、源／目标action3、OffsetX18（十六进制）。
- `1e`帧4/ticks0才建+100→12dd4、pose0c；只用横速高word作X偏移，子速度／Power保留旧值。
- `1f`帧4/5常量40000、6为20000、7为10000位移，每4ticks残影，不覆盖父VelocityX；帧4 contact.bit0后先清ticks再帧++。
- `20`仅frame5/ticks0/1/2射三次，Gravity高word游标按100／300→0轮换，低word保留；先改纵速／游标再查busy。顺序+200/+300/+100，12c96姿态13，Y−34/X±38、纵速0/−30000/+30000、属性最终OR2000，不能直接使用幻海OR4000初始化（十六进制）。
- `12dd4`初始化夹取X高word但保留小数、不跟随或按速度移动；初始化与生命255先于pause，旧FrameIndex/Flags/Completed保留。姿态9/ticks0执行1fce8成对尘土。
- 非零HitStopTicks先递减，再按新奇偶抖动ScreenOffsetX，冻结时钟但仍提交形状／投影；不得调用人物零停顿前置引入其他逻辑。
- 补并验证公共动画姿态9/12/13；原子缺库／游标／帧错误、完整父三子FX/throws/shapes/声音/RNG及真实时钟连续到删除。

规则覆盖、资源存在、原自然输入和新建对局分别验收。当前未新增来源2玩法入口；支持范围继续以 PROGRESS 为准。
