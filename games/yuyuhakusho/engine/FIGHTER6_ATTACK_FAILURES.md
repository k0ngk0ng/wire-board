# 来源 6「阵」攻击选择：实现前失败场景

本清单先于 `fighter6_attacks.go` 和对应测试建立，工作基线为 `8c2ed877`。范围为 `1738c / 174ce / 17654 / 176be` 四个选择器及其立即初始化。静态依据为现有直接 Ghidra 清单；REA 不支持本目标的 raw 68000 路由，不将清单称为 REA 反编译结果。

API 为 `Fighter6AttackState{ChuAttackState; AlternateVelocity int32}` 与 `SelectFighter6Attack`。AlternateVelocity 只视图化既有 Aux48 long；`19498 CLR.W +48` 清 big-endian 高 word，保留 low16，不建立另一份 canonical 存储。

## 普通选择器

| 入口 | 失败场景 |
| --- | --- |
| `1738c` 地面 | Request 8 优先；Request 7 且 Pose≠11 才快移；前快移 Action=`0e`，两方向 VX=`40000`；方向表 16 项；B 优先；横向 A 的投技后必须进入 `13/83`，下向 A 进入 `13/86`，两者完整写 VX=`40000` 并更新 Facing |
| `174ce` 空中 | 先运行额外投技，再处理 Held C + Pressed B 强写 Request 8，再尝试成本 8 的后／前快移，最后才普通方向表；不能合并、提前返回或撤销前面的合法副作用 |

- 空中额外投技仅排除方向 0／1，Pressed A 即提交；B+A 同时输入仍提交投技，后续普通攻击仍以 B 为先。
- 空中 Request 8 能量不足时保留 Request 8；若同时已提交投技，throw 缓冲和游标仍保留。后续可进入普通攻击或无操作。
- 空中 Request 7 的 Pose=11 抑制快移；成功为 `0e/ae`。Request 8 成功为 `11/0c`。两者不分配地面 FX，也不清 Request 或未写的 EnabledStages。
- 地面投技：X 原 word、Y−`22`、W=`18`／H=`22`、SourcePose=`b8`／Action=3、TargetPose=`40`／Action=3、TargetFacing=0、OffsetX=−`18`。空中投技：X±`10`、Y−`10`、W=`0c`／H=`10`、SourcePose=`b8`／Action=9、TargetPose=`41`／Action=9、TargetFacing=0、OffsetX=−`18`。检查 Facing 任意非零、坐标 word 回绕、全部 throw 槽及游标。

## 特殊选择器与七个根

| 根 | 入口条件 | 成本 | 结果与失败场景 |
| --- | --- | --- | --- |
| `19474` | 地面 Request 3；空中 Request 3 另有高度门槛 | 8 | `33/ac`；VY=`-60000`；Aux48 高 word 清零、low16 保留 |
| `194aa` | 仅空中 Request 4 | 8 | `34/ad`；VY=`60000`；地面 Request 4 必须无操作 |
| `19518` | 两入口 Request 1，Changed B | 16 | `35/a0`；B 优先于 A |
| `19542` | 两入口 Request 1，Changed A | 24 | `36/a2`；不足成本或缺键时保留状态 |
| `1959a` | 两入口 Request 2，Changed B | 24 | `37/a4` |
| `195b2` | 两入口 Request 2，Changed A | 32 | `37/a5`；B 同时变化时仍选前一根 |
| `1956c` | 仅地面 Request 6，Health≤111，Changed B+A 同时为 `0x50` | 200 | `38/a8`；VX 清零；单个 B 或 A 不接受；空中 Request 6 无操作 |

空中 Request 3 比较 signed word：先将 Ground 高 word 减 `0x80`，按 word 回绕，再判断结果是否大于 Y 高 word。覆盖高度边界 −1／相等／+1、signed 极值及坐标小数。不能替换成 int32 的世界坐标距离比较。

既有已验证命令表中 slot 3 与 slot 5 同用 `1eab7 [2,0,2,ff]`、window=8，slot 5 后执行。相同清零初态的 DD 输入最终输出 Request 6，覆盖先前 Request 4。空中 Request 4／`34` 的私有分支覆盖不能冒充真人 DD 自然触发；不改命令表、不向自然流写 Request。实际 AI 可直接给出 Request 4，是否自然到达由主流程另行取证。

所有成功特殊构造清 Request、EnabledStages 并执行 `19c1c` 重置，保留 Facing。没有 ChildSize 门槛，不应套用其他来源的 child 阻断。Energy 覆盖 `-32768/-1/0/cost-1/cost/cost+1/32767`，Health 覆盖 `-32768/110/111/112/32767`，Changed 覆盖全部 byte，Aux48 覆盖非零高低 word 及 signed 极值。

## 完整性、错误与连续性

- 四个对象地址执行原 CPU 完整入口至调用者返回，核对 `exit_sp=entry_sp+4`；不能在构造入口或子调用 RTS 截断。
- 比较完整 actor、三个 child、其他 world 对象、全 16 槽 FX、512 字节 throw、两个游标、位置小数、Energy fraction 和共享 RNG，不屏蔽差异。
- 地面快移覆盖空池、当前槽可用、扫描后槽可用、满池、游标回绕与高位游标，保留完整分配副作用。
- 不支持模式、实际访问到的非对齐 FX 游标／throw 游标、无可用 throw 存储须返回原状态及错误，不改共享 slice。空中先投技路径的存储错误必须整体原子失败；成本不足是合法 fallthrough，不能按错误回滚。
- 未访问的无效 Pool／Throws 不应触发校验；空中快移不读取 FX 池，特殊选择器不读取 throw 缓冲。
- 单初态连续流仅改变声明输入；另设能量耗尽的空中快移流与连续投技流，不注入原版中间状态或 RNG。
- 唯一合法 ordinary／special 原 core 基线各 3602 帧，3600 帧以后只排空已有调用；被动观察逐帧保持 RAM／video 一致。所有恢复显式使用 `restore_refresh_cycles=True`。私有合成字段不证明自然游玩或角色资源初始化。

先保存 CPU 夹具，再冻结完整字节比较测试并记录实际缺失符号 red；随后才新增独立生产文件。不修改旧 Go、bundle、PROGRESS，不提交或开启新工作者。
