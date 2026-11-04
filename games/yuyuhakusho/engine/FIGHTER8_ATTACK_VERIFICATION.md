# 来源 8「戸愚呂(兄)」攻击选择：实现前验证范围

姓名更正：本阶段最初沿用当时写反的兄弟映射，将来源8误称为「户愚吕弟」。Root放大原角色名表实际字形并核对造型后，确认来源8为「戸愚呂(兄)」、来源10为「戸愚呂(弟)」。本次只更正当前说明与名称metadata，保留这段历史误名记录；来源ID、ROM、Go、冻结测试、原CPU字节、初始失败证据和两份基线均未改变。下文历史发现记录证明当时的选择与Tag8，其旧姓名标签不作为本次姓名纠正的独立证据。

本阶段只恢复 `17adc/17c1e/17c94/17d5e` 四个选择器的真实 RTS 后 SP+4 返回，包括十个特殊构造根 `197c2/197ec/19804/1981c/19846/19860/19896/198b0/198ca/198fc`、普通共用构造、地面投技和冲刺 FX。自然输入由父任务提供；在真实菜单输入和基线可用前不生成自然证据。持续动作、子对象、动作尾段和世界验收由独立阶段处理。

## 实现前列出的失败场景

来源 ID、入口或模式错误；只观察构造而未等真正返回；返回地址或 SP+4 错误；前后冲正横速 `0x34000`、动作 `0f/11`、请求8／请求7与旧姿态`0b`优先级错误；FX方向使用朝向非零与位0混淆；池满、游标回绕和旧槽残留丢失；普通16方向相对表基准、方向分类或B／A优先级错误；地面投技Y−`1c`、宽`14`／高`1c`、源／目标动作3、目标40、OffsetX18／Y0或坐标word回绕错误；空中普通误加其他角色的投技前置；Pressed／Changed／Held／RawHeld或扩展byte混用；地面请求1仅要求中立方向却误加Changed／子槽门槛；请求2的A>B>C优先级及成本24／16／8错误；请求3的B>A、成本均16、完整横速40000／80000与纵速全清零错误；地面请求4主槽占用、A>B>C优先级、成本8、横速b00000／700000／300000错误；请求5误执行；请求6生命111／A Changed／成本200边界错误；`CLR.W (0x48,A5)`误清成低word或整个long；重复新增counter存储；特殊构造擅自转向；能量低word、子槽、Aux48未拥有部分或共享RNG被改写；重置误清请求、命令、动画标志或未知字节；未知模式、实际FX游标／投技存储错误导致部分更新或调用者slice泄漏；逐调用回灌期望中间态伪装连续流；将私有矩阵或自然无动作返回称作自然完整招式／对局。

## 接口与字段归属

固定接口为 `Fighter8AttackState{ChuAttackState; AlternateVelocity int32}` 与 `SelectFighter8Attack(state, ChuAttackMode)`。Body没有`+48`所有者；调用者已有 `ChuActionState.AlternateVelocity ↔ WorldObject.Aux48` 完整long，本状态只建立视图，不能新增同一counter的另一份canonical存储。

MD大端下 `1991c CLR.W (0x48,A5)` 清完整Aux48的**高word**，保留低16bits；后续`40`的`1c298/1c29e`也读写同一高word。该构造同时清横速完整long。初始静态清单来自已有直接Ghidra输出，REA不支持raw68000，不称作REA已反编译ROM。

## 取证顺序

先保存失败清单和接口，等待真实菜单普通／特殊输入后，生成并共享唯一原core3602帧基线；观察core逐帧完整RAM／视频一致后，采集自然、私有代表、边界和单初态流。全部夹具与真实缺符号测试红灯先于独立Go生产代码。

比较父体256 bytes、三个子槽768 bytes、四人物家族4096 bytes、FX2048 bytes、投技512 bytes、全部相关游标、能量低word、完整Aux48和RNG。私有调用核对完整RAM允许写入范围，恢复检查点始终使用 `restore_refresh_cycles=True`。自然与私有分开统计，不修改既有Go、bundle、PROGRESS、其他报告、页面资产或共享缓存。

## 已验证的普通自然与私有范围

真实菜单输入现已由父任务完成：[source8-next-inputs.json](../../../docs/research/md-yuyuhakusho/source8-next-inputs.json)，SHA-256为 `e70b955c9be3e197fd5ab91189e0621c6e8d251987a5a1d57cdffbf3f14ffb76`；[发现记录](../../../docs/research/md-yuyuhakusho/source8-next-selection.json)确认实际选择与Tag8；其历史误名按开头说明保留。未观察原core冷启动[3,602帧参考](testdata/fighter8-attacks-reference-frames.json)和本阶段观察core全部RAM／视频帧一致，前3,600帧入口取得1,972次自然完整返回，末两帧仅用于等待返回。

| 范围 | 完整调用／步骤 | 覆盖 |
| --- | ---: | --- |
| 普通自然 | 1,972 | ground922、ground-special948、air51、air-special51，均为source8／对象8400 |
| 私有代表 | 31 | 四入口、十特殊根的成功／失败、地面投技及冲刺 |
| 私有边界 | 14,968 | 普通16方向、全部输入byte、成本／生命／占用、池游标与Aux48六种word图案 |
| 单初态流 | 16条／320步 | 每种模式四条朝向流，只改变预定基础Held／Pressed／Changed |

普通自然实际命中 `18828`两次、`18812`一次；没有从这些调用外推十个特殊根的自然覆盖。

私有阶段早于菜单解决，使用已验收的旧source2 frame2400核心检查点，实际执行原 `126c8`、来源byte设8，取得来源8图形集合、图块基址、动画集合、形状集合及移动参数五个指针，全部与先前原CPU初始化目录一致。所有私有样本使用这些真实资源指针，未使用伪造RAM参数表。[初始化记录](testdata/fighter8-attacks-initializer.json)保留原输出。该阶段明确 `private_context_verified=true`、`frames_match_original_core=false`，没有制造source8自然基线或把旧角色上下文称为自然游玩。

全部正常调用核对实际RTS后SP+4、完整父／子／四人物家族／FX／投技／游标／能量小数／Aux48／RNG，无字节屏蔽、未完成、采样错误或私有范围外RAM写入。RNG实际调用数为零。测试与夹具先于生产，保留只缺 `Fighter8AttackState/SelectFighter8Attack` 的[真实编译失败](testdata/fighter8-attacks-red.jsonl)。首实现的六项顶层测试（旧酎攻击三项、source8三项）原生竞态74.433秒、Wasm17.615秒全部通过。

## 十个特殊根与不同的优先级

成本为十进制，地址／动作／姿态与速度为十六进制。

| 模式／请求 | 根 | 成本／门槛与结果 |
| --- | --- | --- |
| 地面1 | `197c2` | 24，中立方向，不要求Changed／子槽；`03/a0` |
| 地面2 | `197ec/19804/1981c` | C8／B16／A24，但检查顺序为A>B>C；`03/a1、a2、a3` |
| 地面或空中3 | `19846/19860` | B>A，成本均16；完整横速40000／80000、纵速全清0，`3e/a4` |
| 地面4 | `19896/198b0/198ca` | 主槽空闲，A>B>C，成本均8；横速b00000／700000／300000，`3f/a7` |
| 地面6 | `198fc` | 生命≤111、A Changed、200；横速完整清0、Aux48高word清0，`40/ac` |

请求5和其他空中特殊请求保持。特殊构造都不重新转向；普通地空16项方向表仍逐项处理。地面前向A记录为X不偏移、Y−1c、宽14／高1c、SourcePoseb8／SourceAction3、TargetPose40／TargetAction3、TargetFacing1、OffsetX18／Y0，再进入父03/83；空中普通没有投技前置。前后冲都使用正34000，分别0f／11。

接口只携带现有完整Aux48的AlternateVelocity视图。`40`构造的结果为 `old & 0x0000ffff`；矩阵区分清高word、误清低word及误清整个long。能量低word和其他未写字段保留。未知模式、实际使用的FX游标／投技记录存储错误整体返回初态，调用者slice不变；不把无效硬件地址别名扩成RAM解释器。

## 特殊自然追加与最终结果

父任务随后提供唯一固定 [fighter8-special-inputs.json](testdata/fighter8-special-inputs.json)，SHA-256 `b26515ad1e9f558fc54699fd0fe69c7c1cf7e3e081682f136bc8da1121121cce`。未观察原core冷启动生成并共享[3,602帧参考](testdata/fighter8-attacks-special-reference-frames.json)，本选择器观察器全部RAM／视频帧一致；只采前3,600帧入口，末两帧drain。

独立特殊自然夹具取得1,358次完整返回（ground677、ground-special679、air／air-special各1），实际成功进入 `198b0`@1816（223→215，`3f/a7`）、`198fc`@2652（215→15，`40/ac`）、`19846`@2909（27→11，`3e/a4`）。只据此确认三个特殊根的自然成功，其余由私有矩阵覆盖，不外推十根自然全覆盖或完整招式。

实现前编写的相同原字节断言直接纳入独立special-natural文件，测试与生产代码未再修改。最终六顶层测试包含普通／特殊两份自然、代表／边界／连续及旧酎三项回归，原生竞态80.000秒、Node/Wasm18.712秒全部通过。首轮74.433／17.615秒日志也保留独立来源。[验证报告](testdata/fighter8-attacks-validation.json)绑定全部输入、参考、旧检查点上下文、真实初始化、五组夹具、红／绿日志和代码摘要。

## 复跑

从worktree根目录运行，固定研究工具按[说明](../../../scripts/yuyuhakusho/README.md)准备。保留旧检查点私有路径与真实自然路径的区别：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter8_attacks.py \
  --out .local/md-yuyuhakusho/fighter8-attacks-private-replay \
  --private-context games/yuyuhakusho/engine/testdata/fighter2-attacks-diagnostic-context.json.gz
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter8_attacks.py \
  --out .local/md-yuyuhakusho/fighter8-attacks-natural-replay --natural-only
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter8_attacks.py \
  --out .local/md-yuyuhakusho/fighter8-attacks-special-replay \
  --inputs games/yuyuhakusho/engine/testdata/fighter8-special-inputs.json \
  --baseline games/yuyuhakusho/engine/testdata/fighter8-attacks-special-reference-frames.json \
  --natural-entry-limit 3600 --natural-only
```

特殊输出中的 `fighter8-attacks-natural` 对应canonical `fighter8-attacks-special-natural`，不得覆盖普通自然。私有 `126c8` 初始化及每次恢复都使用 `restore_refresh_cycles=True`；资源指针来源是原CPU实际输出，不是被选择器读取的原ROM之外伪参数表。

最终聚焦验证使用19个源码／测试文件，所有缓存与日志留在worktree：

```sh
task_root="$PWD"
mkdir -p "$task_root/.local/md-yuyuhakusho/tmp"
(
  cd games/yuyuhakusho/engine
  GOCACHE="$task_root/.local/go-cache" GOMODCACHE="$task_root/.local/go-mod" \
  GOTMPDIR="$task_root/.local/md-yuyuhakusho/tmp" \
  go test -race -count=1 -run '^Test(Fighter8Attacks|ChuAttacks)' -json \
    input.go fighter_input.go animation_clock.go animation_records.go ground_actions.go \
    motion.go jump.go effects.go throw_contacts.go throw_records.go chu_attacks.go \
    fighter8_attacks.go attack_contacts.go effects_test.go jump_test.go \
    throw_contacts_test.go attack_contacts_test.go chu_attacks_test.go fighter8_attacks_test.go \
    > "$task_root/.local/md-yuyuhakusho/fighter8-attacks-native-replay.jsonl"
  GOOS=js GOARCH=wasm GOCACHE="$task_root/.local/go-cache" GOMODCACHE="$task_root/.local/go-mod" \
  GOTMPDIR="$task_root/.local/md-yuyuhakusho/tmp" \
  go test -count=1 -run '^Test(Fighter8Attacks|ChuAttacks)' -json \
    -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" \
    input.go fighter_input.go animation_clock.go animation_records.go ground_actions.go \
    motion.go jump.go effects.go throw_contacts.go throw_records.go chu_attacks.go \
    fighter8_attacks.go attack_contacts.go effects_test.go jump_test.go \
    throw_contacts_test.go attack_contacts_test.go chu_attacks_test.go fighter8_attacks_test.go \
    > "$task_root/.local/md-yuyuhakusho/fighter8-attacks-wasm-replay.jsonl"
)
```

单初态流只推进选择器，不推进动画时钟、持续动作、子对象或碰撞；消费请求后可能保持无动作。自然均为来源8真人，没有来源8自然AI覆盖。私有初始化、selector规则、资源、完整父子世界和可玩入口分别验收；本阶段不称作完整动作／世界／对局完成，也没有修改其他阶段的证据或页面资产。
