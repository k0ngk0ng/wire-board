# 来源6「阵」完整组合：实现前边界

本子任务只新增input／action／active actor原CPU完整组合证据、六项先行测试、规范夹具与只读Root接线复核，不写生产、不改bundle／PROGRESS。边界为15540完整输入返回、15a18→15b82完整动作及12682→126ba人物规则。合法菜单／特殊输入由Root提供，selector生成唯一原core3602基线；不重复control，不将自然human说成自然AI。

实现前失败场景：仅改tag而沿用其他来源参数／动画／形状；33～38及0e／11／13的分派错误，或把selector不产生的0f共享私有上下文称自然可达；Air普通额外投技后，Req8写入／能量8失败／继续普通选择的副作用丢失；投技SourceAction9和TargetAction9被普通父动作覆盖；33 Aux48高word计数／低word保持错误，16门槛或转air顺序错误；34落地后ExtraJump清零遗漏；35／36只按Y／Ground高word恢复的规则被替代；37无条件air完成恢复被误作站立；38 StunTicks原word owner被复制，signed256／新ABC／128末恢复门槛、18682→0c/2b及Aux7E←Aux80混淆；共享动作的Facing位／任意非零、完整速度与部分字段、真实clock阶段顺序错误；父三子、FX Phase／Duration、资源分数、投技／形状、HUD／声音／共享RNG丢失；canonical A5与24-bit别名混用，栈覆盖统计；无实际音频PC／地址便屏蔽队列；晚期动画／录制／HUD错误泄露前置修改；单初态流回灌期望中间态。

接口为Fighter6AttackState携带既有Aux48 view，以及Fighter6MotionState{ChuSpecialMotionState;StunTicks uint16}；只映射既有canonical，不新增持久副本。无新child。复用旧Chu原字节snapshot／适配器，独立wrapper补可见child+65。四私有A5为FFFF8400／FFFF8800／FFFF8C00／FFFF9000，栈在FFFF00下，restore_refresh_cycles=True；完整调用另审计全部64KiB RAM。连续流只初始化一次，以真实ROM动画推进，仅交付声明外部输入，不喂期望人物／动画／FX／资源／RNG，不调度子对象。

六测试先于shared生产冻结，自然Action／Actor取得actual unsupported6红灯后才通知Root开门；显式file-list可排其他未完成测试，不排旧生产。0f仅作为已有共享原routine的私有直接调用覆盖，不能据此扩大自然closure。静态入口来自直接Ghidra68000，不称作REA反编译。

## 已取得的普通及私有完整证据

普通真实菜单输入`source6-next-inputs.json`的SHA-256为`61b140b4ac6820bb6d352bfc98202129fedc4b310367a055df50e1aaffad319e`。selector生成唯一未观察原core3,602基线，本采集器冷启动观察完整RAM／视频逐视频一致，只接受不晚于3,600的入口，末两步排空完整返回。三层各2,067次，是同一调用链的嵌套边界，不能相加当作三倍覆盖；自然为真人／开局禁用，没有自然来源6 AI。

有界private Action／Input／Actor分别189／32／44次，覆盖24个相关槽和四个canonical身份，全部64KiB RAM改写审计未知写入为零。父三子、FX Phase／Duration、其他人物资源、投技／形状、HUD、能量小数、声音与共享RNG逐字节比较；本来源不建立新child。0f只是明确共享私有入口，不以此证明selector自然可达。

Air同call副作用已完整取证：HeldC加PressedB/A先提交SourceAction9的32-byte投技，再强写Request8；能量8成功进入11且能量0，能量7失败仍保留Request8／投技且能量7，继续普通Air动作。Request7成本8进入0e时也保留此前投技；不把投技生产和父体最终ActionKind混为一份值。

16条完整动作单初态流共1,162步：空闲32，0e／私有0f／11各32，13的83／86为38／40；33恰16步恢复Air2，34三步落地Action0并清ExtraJump；35地／空各44恢复0／2，36地／空各70恢复0／2；37 a4／a5为77／112且无条件Air2；38超时372步进入0c／2b，声明C结束148步站立0。StunTicks／Aux7E按原终点保持，不强求所有流站立。另有Input3流48步、Actor4流96步，明确AI／record／playback／暂停／慢动作上下文；34有原AI执行的private完整Actor，但初态已声明34，不能说AI选择出了34。

六测试SHA-256始终为`9eae88248cb802ad8c7da885ff36347b099c9f03230852d9b0f028264447d83f`。普通自然夹具到位后先运行已有OriginalActions／OriginalActors，实际取得unsupported6运行错误，原日志／初始生产SHA／显式文件列表保留，未排生产文件。此时只声称执行这两项，不把尚缺special的其他四项说成已运行；Root收到后才开shared接线。

## 特殊human自然范围

统一`fighter6-special-inputs.json` SHA-256为`686cbd1c0015aa9b0d73d9b92db8dd4b34d1bfef75327a2261f3d82f2fb6f114`，selector生成唯一未观察原core3,602基线。本观察采集完整RAM／视频逐视频一致，special三层各2,067完整返回，本边界动作计数为13=39／33=16／35=60／36=71／37=75／38=212。MAIN／Support子入口与完整动作计数不同，不能混为同一统计。38在accepted入口≤3600窗口内仍活动，没有自然退出证明；完整372步私有clock终点以及Root后续更长world是独立证据。

普通与special的来源6均为human／禁用，AI调用为0。human严格同零状态DD表槽3／5重合，后槽覆盖会得到Request6，因此本模块没有伪写Request4并称自然34。34保留声明private、真实clock及原AI实际执行的完整Actor上下文；该Actor初态已声明34，不代表AI选择出了34。真正电脑6自然若有则应另作AI-natural/world，不混冻结human六测试。

普通／special动作共83行、人物共113行有实际音频消费者清槽标记，仅37c56／37cac且地址F80A..F80D才排除该行四槽队列相等；其他规则及未受影响队列继续比较，不向Go喂期望队列，private流没有此排除。普通最终producer重采与红灯CPU gzip逐字节相同，仅metadata采集器SHA更新。

本子任务只清理64个重复／中间文件，共947,162,588 bytes。完整private采集附带较大的自然库，清理前核对自然调用区字节摘要与规范natural-only库相同。规范夹具、红灯／metadata证据、两组原基线／输入、ROM／工具、共享缓存及8799服务素材保留。原动画记录由CPU实际选择独立采集，不依赖Go bundle。

原CPU复跑分三类：默认命令复现private／sequences；natural-only复现规范普通自然；special命令复现human特殊自然。不同库联合产生的gzip不能混用摘要。

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter6_composition.py \
  --out .local/md-yuyuhakusho/fighter6-composition-replay
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter6_composition.py \
  --out .local/md-yuyuhakusho/fighter6-composition-natural-replay --natural-only
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter6_composition.py \
  --out .local/md-yuyuhakusho/fighter6-composition-special-replay \
  --inputs games/yuyuhakusho/engine/testdata/fighter6-special-inputs.json \
  --baseline games/yuyuhakusho/engine/testdata/fighter6-attacks-special-reference-frames.json \
  --baseline-manifest games/yuyuhakusho/engine/testdata/fighter6-attacks-special-reference-manifest.json \
  --natural-label special --natural-only
```

## 最终组合与只读接线复核

Root在真正自然unsupported6门禁后完成接线。本子任务实际运行未修改的六测试，首轮全部通过：native `-race`48.300秒、Node／Wasm11.544秒，包含完整原状态／原字节、真实动画流和真正晚期错误回滚；没有生产或冻结预期修正，日志规范保留。

只读最终接线review未发现问题：Action／Actor／World／session显式允许6，0e／私有0f／11／13／33～38均按来源6路由到统一Motion；其他来源分支保留。`stepFighter6ActionMotion`先合Base，再显式回写`state.StunTicks`，由已有Actor／World的+84 owner往返，未新增计数副本；Aux48仍单一long，tail完整传回高word游标，低word保留。此前来源1的Power临时通道不变，各错误仍返回原值。

Root的真实AI场景新增`a01e`等待边界只接受id8000／原header Stage32及manager f2a4，旧9ad0仍只接受Stage20；代码不把raw Stage32改成20。此只读review核对准确分支和owner，不以此替代Root独立AI-natural/world证据。ChildAttackFlags及+65保持只读；无新child、无额外子生命周期或存储。

完整来源、12组夹具／manifest、冻结源码及实际红绿日志、当前生产SHA和清理记录见`fighter6-composition-validation.json`。本范围仍止于人物规则精灵提交前，完整world/fresh/浏览器／网络是Root外层门禁。

独立规则复跑不执行模拟器：

```sh
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -json -run '^TestFighter6Composition' ./games/yuyuhakusho/engine
GOOS=js GOARCH=wasm GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -count=1 -json -run '^TestFighter6Composition' \
  -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/yuyuhakusho/engine
```
