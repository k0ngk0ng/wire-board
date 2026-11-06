# 已消除 parser 工作的 sidecar 失败场景

本 revision 仅研究新的不可变 sidecar，绑定正式 schema 2 解压 JSON 的 SHA-256；不修改 Go、catalog、payload 或既有 42 goldens，不新增大 CPU 捕获。

- 将旧候选、压缩文件或重排 JSON 的 SHA 冒充指定正式原 JSON 身份；event key 与正式 prefix／cycle 索引不一致。
- 把原 PC、ROM offset、原控制字节、跳转目标、原栈／计数器镜像或逐帧时刻带入 sidecar。
- 把原 F6～F9 改名为可执行 bytecode，重新保存其控制目标或动态解释源计数器；sidecar 只能描述已展开有限路径的语义工作。
- 有限 loop 首次、taken、exit 或共享 loop／return 字别名被忽略，call／return 顺序被折叠或用另一份状态重演。
- 因 cycle 首次与重复入口的前导工作不同而任选一圈固定化；必须证明 key 足够，或提出显式拒绝／逻辑相位补充，不注入原时表。
- 显式音高、duration-only、duration lookahead take／reuse 混同；复用时值却漏掉实际 lookahead 读取，或给源未读取的字段加读次数。
- F0 参数的 delay／rate／delta／steps 顺序错误；E4 最后同一 operand 的 period／elapsed seed 两次读取被合并成一次。
- 原 token 派生两个 typed event 时重复计算 fetch／参数；或把终结、下一循环前的有限工作留在无 key 的尾部。
- 把 claimed／tie／gate、envelope／pan／modulation 开关、hardware feedback、refresh 相位、bus 等待、IRQ 或 chip busy 写成 sidecar 的第二份动态 owner。
- 合计 cost 隐藏了原有有序工作，使未来 refresh 边界无法逐项推进；或对未知 Work 默默按零成本处理。
- 静态参数顺序和有限路径证明被表述为完整主时钟、原自然执行或 DSP 完成。

验证先保存缺 sidecar 与实际参数顺序错误的 red，再从既有原 flow/token 读取和原 ROM 提炼。原路径、原字节和原 native 序列只进 private provenance。当前最小 observed Operation 层由其他 owner 实现，sidecar 作为后续独立版本。
