# 藏马与七角色世界：实现前验收范围

在 `b5723d24` 六角色检查点恢复来源 1「藏马」。合法菜单输入冷启动 1,800 步已确认 tag1／对手 9。普通原世界双内核 5,500 视频步 RAM／画面一致，共 2,955 轮、38,770 次回调，至标签 4,755 清场。特殊流另从合法固定物理输入采集；不写 RAM 制造自然。

实现前失败场景包括：四选择器仍套别的角色、Power 构造清零遗漏、空中特殊原 RTS 被添加状态校验或副作用；1c 的 Power word 游标／三子顺序／busy 副作用错误，四 FX 的 Phase／Duration 漏回写，旧 child.X 被覆盖，父 X 未进入 child.VX；1a 横速高 word 偏移与低 word 混淆；12dd4 原命中停顿、姿态 9 尘土及清理前缀错误；12ee8 错按 Completed 删除、运动／地面／范围／接触顺序错误；1300c 原点、纵速、相位与寿命新增重复字段或低 word 丢失，波表符号／移位／镜像／归零时机错误，clear 后把旧低 word 合回；新子对象重复推进时钟或读取原来不读的父体／FX／RNG；缺 AN 数据但只靠 tag 显示角色；父子／FX／声音／碰撞调度断裂；从期望中间态补喂整局；真人闲置被自动接管；存档遗漏活跃三类 child 或图块状态；旧六角色状态／事件／像素变化。

先保存失败、原 CPU 夹具与先行测试，再扩共享 input／action／actor、世界回调及对局创建。比较普通／特殊逐轮与单初态连续原世界，三种 child 活动时保存恢复，17 组全新规则配置与 10 组完整呈现；覆盖对全部六名既有人物的正反对战、三人、四人、组队及真人静止。原生／Wasm 全状态／事件／像素和桌面／320px 原生输入须独立留证。

本阶段仍是独立本地研究页；其余角色、场地、声音与统一房间／联机／预测尚待完成，不发布应用或资源。

## 完成的检查

共享驱动已允许来源 1。Power 仍由现有 `ChuActionState.Power`／`WorldObject.Power` 持有，尾段通过临时值入参／回传，所有晚期错误返回原 Power；公开酎包装的签名不变。1c 同时回写三子、Power、Pool 与 EffectExtras，Phase／Duration 保持同一 owner。两个新 child 使用窄视图：OriginX 是现有 VX long，Phase／Remaining 只映射 Aux48／Gravity 高 word，先 canonical clear 后合高 word，低 word 不复活。旧 `12dd4` 继续复用既有规则。

- 普通原世界 2,955 轮／38,770 次回调，至标签 4,755 清场；特殊原世界 1,801 轮／24,333 次回调。两流通过逐轮与单初态连续原字节对照。
- 自然三类 child 共 161 次回调：`12dd4` 67、`12ee8` 22、`1300c` 72，包含 5 次实际 clear。每类 10 个活动状态的保存恢复，与直接分支及原下一轮状态／事件／字节一致。
- 特殊输入自然覆盖四个专属槽；1d 连续 137 帧后恢复站立。六个接受槽共 329 次自然调用，独立私有／真实时钟流另覆盖完整继续路径。
- 17 组全新规则对局、10 组完整呈现跨原生／Wasm 一致；120 份旧对局报告保持世界／事件／生命周期。schema2 仅增加 8 条公共 AN，所需图形和 1,024 项波表复用既有资源。
- 原生竞态与 Wasm 各 265 项顶层检查通过。原生 262 项引擎分片为 88／87／87，另有 3 项动画目录检查，覆盖集合完全相同。桌面／320px 原生输入及截图已检查。

本轮验证器纠正了一个首帧绑定假设：来源 1 两份未改动原 CPU 记录的首轮 `driver_entries=[1800]`，应与已有来源 10 一样显式按首帧一次处理。初始世界失败在该检查中止，原有 fresh／完整组合／CLI／UI 红灯则已实际识别 unsupported1。纠正 helper 后，同一冻结世界测试另在旧 `b5723d24` Go 上实际返回未恢复来源 1；当前实现随后通过两流完整对照。记录、期望规则状态和冻结用例未因此改写，详见[边界纠正](../verification/kurama-driver-entry-refinement.json)及[旧实现回放](../verification/kurama-preimplementation-replay.json)。组合采集器的独立 metadata 变量覆盖错误也完整重采纠正，12 份 CPU gzip 逐 byte 不变，见组合报告。

完整来源、真实失败、最终日志、检查集合、资源和当前源码 SHA 由[七角色验收摘要](../verification/kurama-session.json)绑定。重跑 `sh scripts/yuyuhakusho/verify_kurama_sessions.sh both` 和 `PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/check_kurama_player.py`；原生分片命令保存在[检查清单](../verification/kurama-native-shards.json)。

自然藏马 AI 仍未录制，私有完整 AI 与独立电脑对局另行取证。其他四名角色、七个场地、声音播放和平台房间／预测／联机尚未完成，本阶段不代表整款游戏或发布验收完成。
