# 原球队个人AI 33/34/35验证

实现前失败场景：

- team5分数档2/3及team7非零分数档的原选择表会产生这些模式；不得只放行常量或用其他AI替代。
- mode33的角色奇偶、首次进入、空中、缓存depth zone与高度高/低字节分别判断；height=256..263不等于满足低字节>=8，进入后不能反复触发首次分支。
- mode34首次随机失败只改Mode17，不能提前写Plan17/目标；随机成功后需读取搭档mode40与腾空状态，并按原顺序写搭档Mode/Plan。
- mode34搭档写入必须对后续席位立即可见，不能绕过返回命令直接改view、或给另一席位覆盖无关计数/参数。
- mode34靠近阈值、停止平面速度、跳跃等级3/4、JumpPhase仅FF例外、竖速按无符号字节比较，以及Action3D/3E分支均保留。
- mode35用球动作低3位和原8项概率表，首次失败/成功的随机消费不同；已进入后不重选角色、阈值或计划。
- mode35直接调用AE96，不能套用mode7的HasBall/搭档连接handoff或Counter初始化；跟踪对象6必须读取真实球X，不能改成玩家。
- 缺少被实际读取的搭档状态或原跳跃参数时原子失败；早退分支不要求未读取来源。
- 使用原team selection→个人调用链证明进入条件；隔离矩阵与自然比赛范围分开标注，完整赛由root统一验收。

## 原选择与实现

原球队选择表中，team5在比分档2/3、持球侧的主角色可选mode34；team7比分档非零时，持球主角色选33，非持球/松球角色可选35。这里的档位为原`0696`及队伍侧计算出的档，不是直接使用原始分差。

- **33 / ABEF：** 首入时仅奇数角色、腾空、缓存depth zone<5且高度高字节<2、整数低字节>=8才请求1A、mode6/parameter1和原朝向。已进入不重复机会判断，落地转17；不发射球或改变随机数。
- **34 / AAC7：** 首次原随机高位为1则只退回17。成功才写Plan17/目标搭档/距离4；搭档mode40仍保留，其他搭档按地面/空中分别写mode28或mode0+Plan28。接近、停止平面速度、两种跳跃阶段及Action3E→31均按原分支恢复。
- **35 / AC3A：** 首入的角色奇偶、球动作低3位及难度阈值决定回2或进入AE96参数7、目标球6。继续阶段直接调用共享AE96计划函数，不经过mode7特有的持球/连接handoff。原8项阈值由ROM来源元数据保留。

`AIControlView.TeamAI`只引用本次输入阶段的当前队伍状态。mode34返回`AITeammateControlCommand`，由`world_inputs.go`提交；地面搭档不覆盖原Plan，任何分支失败都不直接修改view或队友。后续席位读到已提交的原命令，不增加持久策略副本。

## 原指令与调用证据

REA `ev_ee1f781ed69881c4b9ef47d01fc03943dcd8af32c6a005d5bcdc83ba43ea83a5`覆盖ABEF/AAC7/AC3A/AE96/B05C/B20C，完整分析位于 `.local/basketball/rea/ai-team-modes/analysis/`。

`verify_ai_team_modes.py`执行原A05D完整个人分派：

| 夹具 | 原调用数 | 重点边界 |
| --- | ---: | --- |
| mode33 | 1152 | 四席、角色0..3、首次/持久、腾空、zone5、高度7/8/255/256/263/264/511/512及负值 |
| mode34 | 1600 | 首次概率/搭档40、地空搭档、跳跃等级/阶段/竖速、3D/3E、接近距离与停止 |
| mode35 | 1920 | 首次奇偶/球低3位/难度/RNG，持久AE96多参数、跳/落地、HasBall及搭档链接不误handoff |

全部4672条比较完整WorldInputState，包含队友记忆、随机数、动作/进入位、Heading/Facing、下一动作、跳跃阶段以及未改世界字段。

另保留224次原`99F0`球队选择，涵盖team5/7两侧、全部比分档、持球/非持球与地空条件，不筛掉未立即进入目标模式的记录。其中80条可继续原目标函数链，共104次个人调用：33为48次、34为8次、35为24次，另24次为真实mode1空中接球→33交接。其余144次保持原选择结果或恢复等待，不伪称完成目标模式链。单独两次连续AI调用还证明mode34的搭档mode28命令会被后执行的席位实际消费。

这些都是显式配置的隔离原函数/选择链，未推进全场物理，不能代替自然原版整局。来源RAM/ROM SHA、输出SHA、原阈值及C000音频驱动排除说明在 `testdata/ai-team-modes-source.json`。

## 验收与限制

Native、race、Wasm同一39项定向测试全部通过，覆盖新增模式与选择链、mode7/3E、既有mode5/plan11、队伍分派及1998次完整输入阶段回归。完整记录、测试集合和源码/日志SHA位于 `.local/basketball/rea/ai-team-modes/verification.json`。

未修改world_actions.go或world_loop.go，也未重复完整比赛门禁。缺少mode34实际读取的搭档状态、无来源目标坐标、无效原跳跃参数或mode35阈值表外难度仍明确失败；早退分支不强求未读取的上下文。其他未恢复个人模式/计划保持原有错误。

```sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_ai_team_modes.py
source scripts/env.sh
go test ./games/basketball/engine -run 'TestOriginalAITeamMode|TestOriginalAITeamSpecialSelection|TestOriginalAIMode34Later|TestOriginalEngagement|TestOriginalAction3EDispatch|TestOriginalAIPossession|TestOriginalPossessionWait|TestComputerActorPasses|TestOriginalCompleteTeamAIPhase|TestOriginalTeamAI|TestOriginalAI(Mode5|Plan11|TacticSelection|ForwardOpponent|ControlAndHoopCollection)|TestWorldInputs' -count=1 -json
go test -race ./games/basketball/engine -run 'TestOriginalAITeamMode|TestOriginalAITeamSpecialSelection|TestOriginalAIMode34Later|TestOriginalEngagement|TestOriginalAction3EDispatch|TestOriginalAIPossession|TestOriginalPossessionWait|TestComputerActorPasses|TestOriginalCompleteTeamAIPhase|TestOriginalTeamAI|TestOriginalAI(Mode5|Plan11|TacticSelection|ForwardOpponent|ControlAndHoopCollection)|TestWorldInputs' -count=1 -json
task_go_root="$(go env GOROOT)"
GOOS=js GOARCH=wasm go test -exec "$task_go_root/lib/wasm/go_js_wasm_exec" ./games/basketball/engine -run 'TestOriginalAITeamMode|TestOriginalAITeamSpecialSelection|TestOriginalAIMode34Later|TestOriginalEngagement|TestOriginalAction3EDispatch|TestOriginalAIPossession|TestOriginalPossessionWait|TestComputerActorPasses|TestOriginalCompleteTeamAIPhase|TestOriginalTeamAI|TestOriginalAI(Mode5|Plan11|TacticSelection|ForwardOpponent|ControlAndHoopCollection)|TestWorldInputs' -count=1 -json
```
