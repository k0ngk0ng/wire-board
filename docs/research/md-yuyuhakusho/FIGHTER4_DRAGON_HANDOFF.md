# 飞影龙／强化子对象交接

基线为 worktree `worktrees/yuyuhakusho`、提交 `605efb55`。本交接只完成只读静态分析；尚无本轮 projectile Go、测试、CPU 夹具或数据导出。Root 已要求停止本 scope 的实现，由 fresh worker 接手；总目标继续。禁止把以下静态推导写成执行覆盖。

## 已保存静态来源

均由既有 Ghidra 12.1.4、`68000:BE:32:default` 直接反汇编，REA 不支持 raw 68000。

- `static/fighter4-projectile.txt`：入口 `13BB6/13FC8` 及可达指令，221 条；另在文件中包含强化负能量构造直到 `14148`。SHA-256 `251c1319f7f58f690ccef144350f2ae80dabf56e7fbd4d86abca3a4b85ceaedb`。
- `static/fighter4-dragon-follow.txt`：`13E9A` 身体段、`1FA9E` 特殊分配器、`1FDFA` 冲击初始化。SHA-256 `6d2d518047205d85f1df7ec874eaad81fbf288f05ac48ae060c3aaf9058219f2`。
- 原父动作复用 `static/fighter-action-slots-focused.txt` 的 `1B2D4/1B3F0/1B48A`。
- ROM SHA-256：`c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06`；被动 core：`e87739d0804e62423fda890377fa46a8932fe1a6fa0132b1ad7f13b8b3af7306`。工具、源码不改；恢复必须 `restore_refresh_cycles=True`。

## 实际回调链与绘制

`13BB6` 自身每次最多一次 `1580`，不是在函数内循环绘制整个龙。它在年龄不超过 31 且年龄低两位为零时，经 `13E5A→1FA9E` 从现有 `9400..9C00` FX 池分配 **新回调 `13E9A`**。身体段也各自进入 `1580`。所以完整规则必须包含第三回调、原地址调度及每次 callback 当时的 `WorldDraw`；不能只画头，也不能回调结束后从最终对象重建历史 Draw。

记录 `1580` 的全局先后序号、A5/ObjectID、来源 callback、当时 gfx／tile／parameter／Screen／GraphicOffset／Attributes／Depth／Layer／SkipGraphics／cache。参考现有 `WorldDraw`，它是值快照。身体段的后续清除／重用不能改变旧 Draw。Sprite cache `2C..3B` 是呈现消费者的后续写入，规则入口／1580前的初始化写入仍须比较。

## 历史布局与新增 canonical

原真实范围 `FF7000..FF77FF`，四 bank：offset `0/200h/400h/600h`，步长 `200h`。每 bank 32 槽、word stride2：

| bank 内偏移 | 内容 | 头写／身体读 |
| --- | --- | --- |
| `000` | X 高 word | word／word |
| `040` | Y 高 word | word／word |
| `080` | 有效 Angle 高 word | word／word |
| `0C0` | Depth | word／word |
| `100` | Layer 高 byte＋SkipGraphics 低 byte | 头写完整 word，身体只读 Layer byte |
| `140` | Status 高 byte | byte；低 byte未写未读 |

每 bank 的 `180..1FF` 未被该链读取／写入；不存成 RAM 镜像。原状态重用不清整个 bank，头只写当前槽，必须保留旧样本。身体不验证保存的 Owner，因此不能自行添加 owner 生命周期校验或在 bank 分配时清空旧数据。

已获 Root 同意的类型名：`Fighter4DragonSample{X,Y int16; Angle,Depth uint16; Layer,SkipGraphics,Status uint8}`，`Fighter4DragonHistory [4][32]Fighter4DragonSample`，由新 worker 在本新模块定义；Root 的 canonical world 引用此类型。

Root 将统一增加而非借用旧字段：`WorldObject.DragonBank uint16` 对应头 `+FE.word`；world `DragonBankCursor uint16` 对应 `A038.word`；world `DragonHistory`。`29` 仅成功创建头后将 cursor `+200h & 7FFh` 并写 child bank。合法 bank 为上述四值；损坏／未对齐别名的独立诊断不得伪装正常 no-op。

## 13BB6 龙头

初始化 `13BBC` 清图形偏移／参数／四 cache，公共 gfx `D8F1A`、tile `240000`、AN `D92CA`、shape `D9A3A`，属性 OR `A03E`，Duration8／Ticks255。保留旧 FrameIndex／Flags／Completed／其他未写字段；HP255先于pause/slow。

正常：清 Shake，`19C96`；`Energy` 高 word作为声音计数，旧零时请求 `F0h` 到 `A030`，然后 `+1 & FFh`。`Gravity` 高 word波相位 `+20h &3FFh`，读 `34000+phase*2` signed WORD，扩展后左移7写完整 VX。读取 Held低byte方向，经16项表把 **Aux48高word** 加减 `10h`，再 mask3FFh；完整 Aux48 加到 VX，形成有效角（含低word分数）。

VX高word决定 Facing.bit0和head pose bucket；有效角 signed word加倍后分别读 `34000/34200` signed WORD，各左移4，X加／Y减完整 long。年龄是 VY高word，先 word++；只有 `int16(age)>=800h` 才回到400h（不是无符号比较）。以 `age&31` 写该 bank 当前样本，Status=0。

- 若 `int16(age)<=31` 且 `age&3==0`，分配身体段。分配失败仍保留池游标正常推进。
- Ground高word≤Y高word 或 Contact.bit3/4：当前 Status=2，执行 `1FDFA` type1D冲击后 `13E4A→3556`。
- Y高word<0，或X越过 Camera左右边界±40h：Status=1，`13E54→3556`，无冲击。
- 否则 `1EB40→156F0→13DFA→1580`。pause/slow直接走同绘制入口，不推进上述逻辑。

最小 state 提案已获 Root 同意：Visual只共用经审计相同的 `Fighter8ProjectileState` 字段，不调用 StepF8；Direction=Aux48完整long，EffectiveAngle=VX完整long；WavePhase/Age/SoundClock 分别为 Gravity/VY/Energy高word；新增 FE bank；Ground、Depth、Layer、SkipGraphics、Held低byte、ContactFlags；共享Pool及只含段初始化所写 Owner/Bank 的16项临时Extras。全部既有 canonical，唯FE/history/cursor新增。

`StepFighter4DragonHead(state,history,env,library)` 返回自身、history、`ChuProjectileOutcome`、error，具体声明由新 worker冻结。低word只由canonical持有，不能在view再存一份。

## 13E9A 身体段

`1FA9E` 与 `AllocateEffect` 共享原最多15槽扫描／先增游标规则，只把Behavior改成13E9A。头写段 Attributes `(head+18h)&9FFFh |4000h`、Depth、Layer、Owner word `+48`、bank word `+4C`；不写段 age。

段 init13EA0只清偏移／参数／cache、设置公共gfx/tile并 OR属性；不设AN/shape/HP。正常：age=`(VY高word+1)&31`，用 `Gravity高word` bank读当前历史槽；X/Y仅写高word，原小数保持；Angle只写VX高word，Depth word、Layer byte更新。属性高byte bit3/4按角设置；GraphicParameter来自8项表。Status1或2都直接 `13FBC/13FC2→3556`，不画；其他状态 `156F0→13FAE→1580`。pause/slow直接该绘制入口。

段 Owner=`Aux48高word` 已有，但本回调不读它；bank=`Gravity高word`，age=`VY高word`，Angle=`VX高word`，均不是新canonical字段。建议窄 `Fighter4DragonTrailState{Header; Draw WorldDraw(唯一ObjectID/图形/Screen等); WorldX/Y/ScreenOffsetX/Y; Angle/Age/Bank uint16}`，不放AN／HP／buffers／pool。history只读。Root必须将13E9A单独分派，保持实际FX地址顺序。

## 13FC8 强化子对象与父体改写

init13FCE公共图形资源、强制 pose19，Duration8／Ticks255，HP255先于pause/slow。正常清Shake／推进时钟，读取父 `A5-100h`：X/Y高word、Facing、Depth、Layer+4给子。每步父Aux64.bit4置1，AttackValue字节左移1回绕。

父HP非负且pose不是0F/10时：Energy高word≥220则完整Energy减40000h，否则完整Energy减5000h（会借位）。无论是否跳过消耗，随后父Energy高word负则触发以下父构造并删除child：MovementSource12578、StatusDelayFF、Aux7E=FFFF、StunTicks100h、action5/pose25、Gravity3800、VY−60000h、VX Facing0为−18000h否则+18000h、EnabledStages/ActionFlags20h、FrameIndex0/Duration8/TicksFF/Completed0；其余原字段保持，不能擅自用ResetActionState。

未退出则 `1EB40→156F0→140CC→1580`；负能量构造后 `14148→3556`。建议 `Fighter4BoostState{Visual;Depth,Layer}` 加 parent view `{Body,Energy,AttackValue,MovementSource,StatusDelay,StunTicks}`，同时返回child/parent/out/error，晚期缺AN必须一起回滚。

## 必需只读数据与未决 CPU 边界

- Steering16 signed words，地址13C9C：`[0,512,0,0,-256,-384,-128,0,256,384,128,0,0,0,0,0]`。
- HeadPose8，13CE2：`[14,15,16,17,18,17,16,15]`。
- BodyParameter8，13F76：`[90,88,86,88,90,88,86,88]`，是直接GraphicParameter，不乘2。
- signed WORD sine/cosine原基址34000/34200；现Arena signed byte表不足，且cos不是严格sin模1024平移。有效角在相加后未mask，不能把lookup强行wrap；Up steering等可能进入前缀。提案 padded Curve FirstIndex=−32、Values1344覆盖34000索引−32..1311，仅是依据可达值范围的静态候选，**必须原CPU实际地址验证后冻结范围**，不把相邻数据假称三角函数。前缀33FC0存在非零word，尾34A00已是其他数据，不能补数学sin。
- 公共AN pose14..18各1帧、pose19四帧，共9条；先核当前目录，若缺只独立补typed AN数据。body参数86/88/90的GFX/tiles先核world sprite catalog，缺口由Root走独立素材证据，不透明fallback。

## CPU 审计／冻结要求

先列失败场景，保存原CPU自然／dirty边界／真实creator-clock-head-body-history到清除流，再冻结所有先行测试并真实own-symbol red，最后实现。普通/特殊Root供合法固定物理输入，selector产唯一original3602参考，复用而不重采控制组。测试必须核对全父三子／FX所有槽／新FE／完整typed历史对应原范围／声音／RNG／每次Draw序号和值，以及全64KiB实际改写。

允许规则输出的地址由实际函数确认：父family8400..93FF所属区、FX9400..9BFF、history7000..77FF中真实六plane、shape A200..A5FF及A004..A00B、A030声音请求、A036FXcursor、MAIN成功建头时A038bankcursor、F80A..F80D队列、声明私有栈（建议FFFF00以下）。不能把未写的history空洞或低byte算作可任意改写。自然IRQ需登记24位FFF000及实际PC FFFFF000，以及RTE FFF09A/F0A8/F13A；每个区外最终byte按原IRQ前后delta顺序串联（如F517的0→2→4），原队列消费者37C56/37CAC单独记，不能掩屏规则结果。绘制消费者cache/硬件队列的排除域须明确且独立留证。

本轮未采CPU或更改旧Go/bundle/PROGRESS。仅新增上述两份静态文本、本交接；私有Ghidra项目为 `docs/research/md-yuyuhakusho/static-work-fighter4-projectile`（已-deleteProject但目录可能空），cache/log在 `.local/md-yuyuhakusho/fighter4-projectile-static/`，可在Root确认后按本scope清理。不要碰其他角色证据、ROM/core、共享Go缓存或活动worktree。
