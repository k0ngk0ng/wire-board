# 仙水合法特殊输入

固定输入为 [source5-special-discovery-inputs.json](source5-special-discovery-inputs.json)，SHA-256 为 `e6ca2bbb16821f13c2508661b3b1b4ebddcad141f40ed7ef99fac0fea2f54af9`。唯一未观察原 core 控制组为 `engine/testdata/fighter5-attacks-special-reference-frames.json`，SHA-256 为 `8f813685d06dfbe1ca8d9615cd027aef04b130790efe5e6dd66b8395d795bed3`，共 3,602 步，入口采样截至 3600，另两步用于排空。

发现脚本根据实际可见状态选择物理按钮，只记录输入，不写 RAM、寄存器、生命、能量、请求或随机种子。记录后的固定输入从冷启动用原 core 和观察 core 独立复放；三个组件的完整 RAM／画面摘要在全部 3,602 步一致，证据与范围见 [source5-special-discovery.json](source5-special-discovery.json)。自适应发现运行到 3788，不把它与固定控制组的 3602 步混为同一验收范围。

| 根 | 成功视频标签 | 原版构造 |
| --- | ---: | --- |
| R1 普通 | 1808 | `19240` |
| R1 下方向 | 1879 | `19284` |
| R3 双下 | 1953 | `19216` |
| R4 A／B／C | 2055／2117／2181 | `192c8/192f6/19324` |
| 空中 R4 | 2247 | `19364` |
| 弱生命 R6 A／B／C | 3179／3335／3387 | `193a4/193bc/193d4` |

十根均实际扣除对应成本；三次 R6 成功时生命分别为 93／70／51。总消耗 206，初始 223，最终能量为 17。弱生命由原电脑正常攻击形成，未做生命或角色注入。

后续构造只自然观察到 `19444` 在 3419 完整返回；不能据 R6 选择成功宣称三个 `31 → 32` 都自然完成。自然记录还包含 `1414e/143e4/146ba` 三种子对象。其他四个后续构造及其他变体的真实时钟／私有证据与完整世界验收，分别见[攻击验证](../../../games/yuyuhakusho/engine/FIGHTER5_ATTACK_VERIFICATION.md)、[MAIN 验证](../../../games/yuyuhakusho/engine/FIGHTER5_MOTION_VERIFICATION.md)及[完整世界](../../../games/yuyuhakusho/engine/FIGHTER5_WORLD_VERIFICATION.md)。
