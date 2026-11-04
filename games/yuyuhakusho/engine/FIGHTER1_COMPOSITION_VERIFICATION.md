# 来源 1「藏马」完整组合：实现前边界

本子任务只新增完整 input／action／active actor 原 CPU 证据、规范夹具、六项先行测试与只读接线复核，不写生产、不改 bundle／PROGRESS。取证边界为 `15540` 完整输入返回、`15a18→15b82` 完整动作及 `12682→126ba` 人物规则。菜单和特殊输入由 Root 提供，唯一原 core 3,602 基线由 selector 任务生成，不重复 control。

实现前失败场景：只改 tag 而沿用其他来源参数／动画／形状；`1a/1b/1c/1d` 与 `0f/11` 路由错误；selector1 清 Power word 没回写到唯一 `+7c` owner；Power 三子游标、`+100/300→0` 及 busy 前 FX／游标副作用错误；`1c` 只在 ticks1 处理的帧组、四 FX 部分成功与仅第 1／4 Phase=8 遗漏，或 Phase／Duration 重复存储；`1300c` 将父 X 写子 VX 却错误覆盖旧子 X、Y 高 word／速度／属性加法被简化；`1a` 的旧子速度／Power保留、X 只用父 VX 高 word偏移；`1b` 的完整 VX、Y−34/X±20与无 mask 属性混淆；`1d` 接触／48 tick／跳帧顺序和真实时钟错误；父三子、FX、其他人物资源、能量小数、投技／形状、HUD、声音及共享 RNG 改写遗漏；canonical A5与24-bit别名混用、栈覆盖统计；无实际 PC／地址便排除声音；晚期缺库／FX／录制／HUD 错误泄露前置写入；单初态流注入期望中间态；将私有 AI 写成自然 AI、把人物规则返回说成完整世界／子调度。

既有 canonical Power、Aux48、Body及EffectExtras足够，不新增副本。接口为 `Fighter1AttackState{ChuAttackState;Power int16}` 与 `Fighter1MotionState{ChuSpecialMotionState;EffectExtras [16]ReactionEffectExtra}`；1c 不写 PendingSound。复用旧 Chu 原字节 snapshot／适配器，独立 wrapper 补齐可见 child+65。四个私有 A5 使用 `FFFF8400/FFFF8800/FFFF8C00/FFFF9000`，栈在 `FFFF00` 下，恢复 refresh cycles 为 true，完整调用审计64KiB RAM。原动画流只初始化一次，仅接受声明外部输入，不回灌人物／动画／子槽／FX／资源／RNG，不推进子生命周期。

六测试必须先于 shared driver 支持来源1冻结，取得真正自然 Action／Actor unsupported1 运行错误后通知Root。显式源文件列表可排其他独立任务未完成测试，不排生产；自然human不得外推为自然AI。名字保持来源1藏马／8兄／10弟，不更改历史证据。

## 已取得的普通及私有完整证据

普通真实菜单输入为 `source1-next-inputs.json`，SHA-256 `cd3dd66e7ec9d50cc89451afdaa98914c5ca9945bb689f4a5f3b4affa326f1ef`。selector 生成唯一未观察原 core 3,602 条基线，本采集器冷启动重跑观察内核，完整 RAM／视频逐视频一致。入口接受到视频标签 3,600，末两步排空完整返回；视频标签不作为逻辑 tick。普通 input／action／actor 各 2,067 次，是同一调用链的嵌套边界，不能相加当作三倍覆盖。自然输入为真人／开局禁用，未观察来源1自然AI。

有界私有 Action／Input／Actor 分别为 163／32／44 次，覆盖20个相关槽、四个canonical身份。全64KiB RAM改写范围审计未知写入为零，父三子、FX Phase／Duration、其他人物资源、投技／形状、HUD、声音、能量小数与共享RNG逐字节比较。`1c` 使用真实旧子X=`1234abcd`，建立后仍保持该X，子VX为父X=`00641234`；四FX成功的原Duration字节为 `[8,142,142,8]`，两次成功为 `[8,119]`，中间旧值不能统一改写为8。Power游标、忙槽及已完成FX副作用保持原顺序。

11条完整动作单初态流共725步，全部恢复站立0：空闲32，`0f/11`为28／32，`1a`空／忙各56，`1b`38，`1c` a0为58、a1空／全忙各90，`1d`无接触152、从frame18声明接触初态93。空槽分别建立`12dd4/12ee8/1300c`，忙槽不伪造成功；后续未调度子对象或注入期望碰撞。另有3条完整输入流48步、4条完整人物流96步，覆盖显式AI及录制／播放与暂停／慢动作。

六测试文件始终为 SHA-256 `f1578032e7209d349310ae86ae0a68163ba3ab0821a87ff91479f6ac0b5c2380`。普通自然夹具到位后先执行已有OriginalActions／OriginalActors，实际报来源1完整Action／Actor unsupported，正式红灯与文件列表保留。此门禁只声称实际执行的两项，不把尚缺special夹具的其余四项说成已运行；Root收到后才开始shared生产接线，测试断言不变。

## 特殊自然输出与保留范围

统一合法 `fighter1-special-inputs.json` SHA-256 为 `4c4ebc2a611db9d98337010cc5d38eea6cabeaf2bf60826387a0f34733a1fd8d`，唯一原 core 3,602 基线由selector生成。本观察采集独立冷启动核对完整 RAM／视频逐视频一致，特殊三层各2,067次完整返回，包含`1c`59、`1b`39、`1a`57、`1d`137次；没有来源1自然AI。父体完整调用内建立`1300c`3次、`12ee8`及`12dd4`各1次，不能与child生命周期回调计数混用。

普通及特殊自然动作共94行、人物共113行有实际音频清槽标记，仅`37c56/37cac`且地址属于`F80A..F80D`才排除该行四槽声音队列相等；其余规则和未受影响队列继续比较，不向Go喂清理后期望队列。私有流没有此排除。原CPU动画／形状库由实际记录选择独立采集，不从Go bundle获取预期数据。

本子任务只清理102个重复／中间文件，共1,423,797,493 bytes，包含下述metadata修正重采副本。完整private采集附带较大自然帧库，清理前核对其自然调用区字节摘要与规范natural-only库相同；规范夹具、两组原基线／输入、metadata故障与修正证据、ROM／工具、共享缓存与8799服务素材保留。本模块止于人物规则精灵提交前，不替代完整世界、子调度、fresh对局或平台浏览器验收。

首次完整Go验证通过两自然流，却在private／sequences夹具门禁报`unverified wholeactor fixture`。原因是本采集器把baseline列表变量`expected`复用为期望child编号；冷启动前置完整RAM／视频断言已实际通过，最终metadata比较却成为列表与整数比较，错误标记false。仅将该局部变量改名后，重新冷启动两自然流及全部private／sequences，12个CPU gzip逐字节不变；重新生成的metadata仅改变producer SHA和原错误的private baseline bool。没有手改flag、生产或冻结测试。失败日志及`fighter1-composition-metadata-correction.json`保留真实过程与旧manifest摘要。

原CPU复跑时，默认完整命令复现private/sequences，第二个命令复现较小的规范普通natural库，第三个命令复现special；不要混用不同库联合产生的gzip摘要。

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter1_composition.py \
  --out .local/md-yuyuhakusho/fighter1-composition-replay
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter1_composition.py \
  --out .local/md-yuyuhakusho/fighter1-composition-natural-replay --natural-only
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter1_composition.py \
  --out .local/md-yuyuhakusho/fighter1-composition-special-replay \
  --inputs games/yuyuhakusho/engine/testdata/fighter1-special-inputs.json \
  --baseline games/yuyuhakusho/engine/testdata/fighter1-attacks-special-reference-frames.json \
  --baseline-manifest games/yuyuhakusho/engine/testdata/fighter1-attacks-special-reference-manifest.json \
  --natural-label special --natural-only
```

## 最终组合与只读接线复核

metadata修正重采后，本子任务实际运行冻结的全部六测试，native `-race` 46.556秒、Node／Wasm 11.558秒全部通过，包括真正晚期动画／录制／HUD错误回滚。两份最终日志规范保存；首次metadata门禁失败日志独立保留，不把这次经历写成未经修正的首轮全绿。Root生产实现与冻结测试未为此改变。

按Root要求只读复核最终接线，未发现问题：Action／Actor／World／session显式允许来源1，专属`1a/1b/1c/1d`及`0f/11`只走来源1分支，旧来源路径保留。tail以临时Power入参／回传值连接`Fighter1AttackState`，每个晚期错误返回originalPower；公开来源9的`StepChuActionTail`签名不变，仍为包装。Action明确接收Power，motion通过`acceptSpecialMotion`及EffectExtras回写，未把Power放入Aux48。

`12ee8/1300c`分别使用窄Visual加自身字段，无父依赖或额外FX池。Wave OriginX映射canonical子Body.VelocityX，真正子X仍由Animation拥有；Phase和Remaining分别映射Aux48／Body.Gravity高word。外层先按实际前缀清canonical对象，再将primitive返回的已清view合并，并在已清long上仅合高word，因此未清低word保留、已清低word不复活。旧`12dd4`继续用专用F2回调，三子初始化不覆盖视图外的动画／攻击标志／停顿。

来源、最终12组fixture、先行红灯、metadata故障／重采纠正、最终日志及当前生产源码摘要见`fighter1-composition-validation.json`。完整世界／子调度／新建对局／平台浏览器属于Root外层验收，不由本模块数量替代。

独立规则复跑不执行模拟器：

```sh
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -json -run '^TestFighter1Composition' ./games/yuyuhakusho/engine
GOOS=js GOARCH=wasm GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -count=1 -json -run '^TestFighter1Composition' \
  -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/yuyuhakusho/engine
```
