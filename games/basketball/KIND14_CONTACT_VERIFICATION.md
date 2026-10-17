# Court3 Kind14 接触验证

实现前失败场景：

- Kind14 的 action map 为原 bank5 `8F95`，常规 shape group9，不能复用 Kind13 的 group8。
- action3 选择 group30，越过30组静态指针域后产生原 RAM 形状读取；不能凭静态数组扩容、改 group 或置空来伪装支持。
- idle 保留先前 cursor，必须覆盖对应原静态 shape 数组全部项；未知 action/frame/kind 仍应报错。
- 非kind10的反向接触仍须原机器码验证人物受击、冲量、Endurance、声音、道具反弹及SpecialHolder写入；不因类型新增而改变这些语义。
- 双action6人工输入的原别名写结果继续保留并明确拒绝，不能删除不匹配记录。
- Native、race、Wasm 使用相同夹具与测试范围；本项不代替Court3完整比赛门禁，也不改动opening/world/runtime文件。

复用 REA `ev_28ff5df98684db5f3fe58534b0506c5cfc3e41fbad9c8a013ccdc808f97f542a` 的8318/876F/87EA，分析文件位于 `.local/basketball/rea/prop-kind13/analysis/`。当前bank5 SHA-256为 `025909ee6b8dcf27215d1d6b183d6e913e878141d11e985e07930a912cb722ec`。

已观察到的不同分支：原8318在Kind14/action3/cursor0..3下分别返回shape指针`0080/0000/0080/0000`；改变显式合成RAM输入后形状字节随之改变。后续原拾取/A投掷函数链证实cursor0/3可达，现已按真实语义来源恢复0..3；不是用静态零形状替代。完整自然比赛可达性仍不能仅由隔离链推断。

## 结果

- 静态几何原函数对照扩展至418条，新增Kind14的78条；原普通形状为X[-8,8]、height[1,11]，并非Kind13的X[-4,4]、height[1,15]。原action0..15中除action3外按源数组完整导出。
- `auxiliary-geometry-dynamic-isolated.json`独立保留8条原action3结果与两种合成RAM模式。无上下文的静态查询仍明确拒绝；有语义上下文时由专门解析器读取实际控制器/坐标。其来源及SHA由配套source.json记录。
- 原反向接触oracle扩展至1596条，新增384条Kind14：受支持状态逐字段匹配，Endurance与旧对象字段保持；原6条双action6人工输入仍保留机器码输出并测试原子拒绝。
- 静态阶段Native、race、Wasm同一13项定向测试通过；增加动态语义接线后，同一21项定向测试在三者均通过，记录为 `.local/basketball/rea/prop-kind14/dynamic-{native,race,wasm}.json`。结果、测试清单及日志/夹具SHA保存在 `dynamic-verification.json`。未运行或修改root负责的Court3完整比赛门禁。

## 动作3入口审计

bank6 `AFED`经`B001`表分派到`B0B1`。动作3入场将动画cursor设为3，读取旧Owner朝向，`EBE9`设VX为±12像素/逻辑步，VZ=-3、重力0，并清Owner/Attachment；后续平面速度为0时设重力1并重置cursor0，落地转idle。故在该动作首帧，Kind14的原接触形状指针为`0000`。指令保存于 `.local/basketball/rea/prop-kind14/action3-source.txt`。

已定位的一个裸写入口为扣篮链`8E9D → 8FBB → 8FC8`，写action3到`056E`所选对象；`0547`高位为1时覆盖成9。该调用链本身不证明目标会选到Kind14，其他间接/请求入口尚未穷尽。复用既有 `.local/basketball/rea/dunk-chain/analysis/` 证据。

## 原拾取与A投掷链

`verify_kind14_pickup_throw_chain.py`以保留的Court3原RAM为基础显式设置人物0与Kind14道具5靠近、站立和未持有。之后只提供控制器采样，依序调用未改指令的`805C`输入、`E18B`请求/动作以及原物理函数：

- 第一次A经`8374/83B9`请求人物4E；`91A4`把道具设1、Owner=0并附着。
- 人物完成拾取进入20；第二次A请求27，`927F`将道具设14。
- 原道具14处理`B42B`设置NextAction=3，动画结束后道具3/cursor0出现；下一次`B0B1`入场变cursor3，并按原速度飞行至落地。

两个链分别保持未分配控制器3的Held为0与`0x41`，每个23次调用周期，产生合计22次动作3接触查询。`kind14-pickup-throw-contact.json.gz`保留各查询的当前世界、控制器、原指针与形状；完整状态序列、来源RAM/ROM/输出SHA另存同名chain/source文件。没有在链中直接写Action、Owner或结果；C000仅作为记录音效的返回桩。这是显式起点的原函数链，未模拟其他人物的完整同时比赛，也不冒称Go端拾取/投掷动作已经全部恢复。

## 动态来源与瞬时接线

原搜索调用`8318`写`002C/002D`，随后`8350/83E2`读取形状。按cursor读取的真实来源是：

- cursor0/2的`0080..0087`：篮筐11/12的X整数低字节，以及人物0..3、道具4..5的X最高字节，全部取当前canonical位置。
- cursor1/3的`0000..0007`：4个控制器Held随后4个Pressed。原`805C`正是从`0000,Y/0004,Y`读取这些端口，不能从actor映射反推，也不是任意未知scratch。

`ContactGeometryContext`只保存瞬时指针，不新增持久RAM数组。`runtimeWorldInput`保留映射前的控制器采样，经`WorldLoopInput.PhysicalControllers`传到`WorldPreparationContext`；位置直接从当前世界构建。未分配控制器输入保留，已知全零输入与未提供来源严格区分。读取是按需的：坐标cursor不要求控制器，控制器cursor不要求位置；cursor>=4仍明确报错。

回放沿既有`StepRecordedVideo → StepVideo → runtimeWorldInput`使用记录中的Held/PriorHeld/PendingPressed；没有修改回放实现或从actorFrames反推。180帧未分配控制器持续输入的直接/回放结果完全一致。原22次链内接触投影逐项匹配；原世界准备阶段回归保持通过。

## 复跑

```sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_auxiliary_geometry_original.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_prop_attack_original.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/export_contact_profiles.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_kind14_pickup_throw_chain.py
source scripts/env.sh
go test ./games/basketball/engine -run 'TestOriginalAuxiliaryGeometryCompleteTables|TestOriginalAuxiliaryDynamicGeometryRemainsExplicit|TestOriginalPropAttack|TestOriginalPlayerProp|TestOriginalCompleteCombatPhase|TestOriginalCompleteInteractionPhase|TestInteractions|TestOriginalKind14PickupThrowContactChain|TestDynamicContactGeometry|TestRuntimeContactPhysical|TestRecordedVideoRetainsUnassigned|TestWorldPreparation' -count=1 -json
go test -race ./games/basketball/engine -run 'TestOriginalAuxiliaryGeometryCompleteTables|TestOriginalAuxiliaryDynamicGeometryRemainsExplicit|TestOriginalPropAttack|TestOriginalPlayerProp|TestOriginalCompleteCombatPhase|TestOriginalCompleteInteractionPhase|TestInteractions|TestOriginalKind14PickupThrowContactChain|TestDynamicContactGeometry|TestRuntimeContactPhysical|TestRecordedVideoRetainsUnassigned|TestWorldPreparation' -count=1 -json
task_go_root="$(go env GOROOT)"
GOOS=js GOARCH=wasm go test -exec "$task_go_root/lib/wasm/go_js_wasm_exec" ./games/basketball/engine -run 'TestOriginalAuxiliaryGeometryCompleteTables|TestOriginalAuxiliaryDynamicGeometryRemainsExplicit|TestOriginalPropAttack|TestOriginalPlayerProp|TestOriginalCompleteCombatPhase|TestOriginalCompleteInteractionPhase|TestInteractions|TestOriginalKind14PickupThrowContactChain|TestDynamicContactGeometry|TestRuntimeContactPhysical|TestRecordedVideoRetainsUnassigned|TestWorldPreparation' -count=1 -json
```
