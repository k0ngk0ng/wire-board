# Source5 子对象失败场景（实现前）

本任务限原版 MD《幽游白书：魔强统一战》日本版的1414e、143e4、146ba，及相邻142b4／1458a停顿入口。ROM SHA-256为c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06。先收集原CPU证据、冻结测试并保留真实缺符号失败，再新增实现；不改变冻结断言求绿。

1. 初始stage=0和已初始化stage；暂停bit7、任意非零slow须仍保留初始化和正确draw边界。
2. AN帧索引、duration、ticks、flags、completed与dirty尾字节；三种callback不能共用错误clock。146ba使用1479c的独立type／delay小表，不走AN／shape提交，不额外写health或shape。
3. 1414e near-ground32与143e4 near-ground64在移动前判断；+48仅最高byte递增，低24bit保留；每四次分配trail20。池满、cursor绕回和15次搜索、旧FX数据/坐标fraction必须符合原版。
4. 移动为full long wrapping；ground和arena/y边界为signed word；上边界、左右+32溢出、等于阈值及±1、负/零VY须按原版先后顺序，不能添加自创重力。
5. pose17/18只在相应callback置Auxiliary64 bit5；AN记录可能随后覆盖该bit，不能错误合并所有Body字段。
6. ActionKind=0及contact bit2通过+fe父对象清+69 HitStopTicks／+75 RetainTicks；Gravity只读最高byte0..3；分别生成反射pose14/15/16或19、team低byte减16、VX/VY/facing bit0处理，直接rts无draw。其它gravity最高byte仍清父字段并继续普通分支。父对象身份错误须原子失败。
7. contact8/16与ground触发FX18／19（后者Y高word减16）并清除；contact bit2优先于8/16且只在ActionKind=0，已反射后不得重复清父/减team。
8. 143e4 pose19按父Held低byte的up/down steering，up优先、±0x60000严格比较边界、多一步±0x4000；其他pose不steer。父Held不是RawHeld(+74)。
9. 146ba跟随自身逻辑ID-0x200，完整复制parent X/Y，VX/VY只读高word到screen offsets；parent pose a3之外清除。表终止先于跟随，暂停先于计数；脏index/ticks与fraction须保留。
10. 142b4／1458a相邻停顿入口递减hitstop（0→255）、shake无符号上限、奇偶偏移及AN／projection；不把静态相邻代码称为已观察的自然入口，保留实际调度未见的范围。
11. 3556按16byte块清除：size0默认64、非整块floor、16..256各前缀；清除必须先ApplyObjectClearPrefix再merge窄view，未清字段/fraction不可归零。超逻辑slot和size1..15拒绝且state/pool/parent保持不变。
12. AN缺key、shape容量边界、未对齐pool cursor、错误callback/parent、146ba表越界均原子失败；不把ROM/CPU或全RAM带入最终运行时。
13. 原CPU每次callback全64KB审计；显式逐条归因IRQ（含24/32位入口）、声音队列消费者37c56／37cac，不得统一mask未知地址。每次draw保存当时对象的WorldDraw。
14. 自然普通／special控制流绑定Root唯一原core输入及RAM／video逐帧hash；无自然callback不得称验收通过。dirty边界与单一初态creator-clock-child直到清除，禁止逐步期望值回填。

静态来源：`docs/research/md-yuyuhakusho/static/fighter5-projectile*.txt`；bounded直接Ghidra反汇编，REA不支持raw68000。静态调用图只说明可达控制流，不代表已观察原CPU执行。
