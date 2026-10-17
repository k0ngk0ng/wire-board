# 原版人物攻击动作验收

实现前失败场景：

- 将同一动作ID直接套用Kunio动画，遗漏原Team×4+Character的variant／handler分派。
- 进入攻击时未写AttackVariant($057A+actor)，后续帧错误重复选择variant。
- 原角色增强选项影响60/64分派，普通表与增强选项混淆；未支持handler静默落入通用拳脚。
- handler3写入跳跃阶段0xE0、初速度／重力加法错误；handler4清processed标记却不通知调度重访。
- 落地前／物理后检查顺序被统一，改变落地、Falling、动画下一动作的时点。
- handler15按旧Pose镜像取水平速度符号，而实现错误改用新FacingLeft。
- handler0/5摩擦饱和、电脑RunSpeed刷新、动画FD/FE/FF循环与反向帧错误。
- 原音效事件被当作伤害，或攻击阶段直接伪造接触／命中／扣血。
- 混淆整世界阶段与单人物调用，将其他对象的字段写入误归因于攻击。

自然采样：既有6000帧比赛中全部60..69人物调用，不筛选未知分支；保留完整调用前后RAM并核对整场RAM一致。测试先于实现。

## 本次结果

- REA 批量证据 `ev_535248b0e94c2c008702b2638bdcd014b1a5ccfad5b795c1c1af22262b189e6e`：32 个分派入口及handler全部解析成功，保存在 `.local/basketball/rea/player-attack/analysis/`。
- `extract_attack_profiles.py` 从同一原始目标提取9个动作×32角色的选择表、原音效表，以及97种variant动画为静态语义数据。运行时不读取ROM或执行6502指令。表来源和SHA-256保留在 `profile-source.json`。
- 499 次原人物调用全部通过：60有77次、61有218次、63有85次、64有3次、65有14次、66有66次、67有36次。62／68／69没有本次自然样本。逐调用核对完整人物状态、AttackVariant、JumpPhase、Revisit，并检查其他对象动作及所有者没有未表示的变动。
- 已恢复handler索引0／2／3／4／5／15／19／24。其他索引、未恢复附着攻击和动作69明确报错。完整静态角色表不等于所有角色玩法已验收；固定Kunio／Rick差异测试和自然角色覆盖分开记录。
- `AttackStep.SoundIDs` 输出原声音ID；攻击动作只修改自身运动及动画，不计算接触或伤害。`AttackVariant` 供下一次碰撞取形状和攻击响应，`JumpPhase` 供下一次跳跃使用。
- 原499次单调用没有改变全局056A／06A5或06D9，也没有改变共享随机状态，记录在 `shared-field-audit.json`。06D9是演员2的NewYorkPassage，已经由人物状态保留；整世界动作阶段其他对象的写入不归因于本模块。
- 测试先于实现。针对性验证使用全部生产源与相关测试，精确命令保存在 `scoped-commands.json`；当时其他工作中的world_actions_test尚无实现，不能用本模块结果代替整世界验收。
- 最终针对性竞态与Wasm执行均通过，结果为 `race-scoped.json/wasm.log`。
