# 原版声音分派只读审计边界

本审计不修改 production、catalog 或既有 claim 产物，不执行 CPU 捕获。新增文件只在本组 `AUDIO_GAME_DISPATCH_ORIGINAL_*` 和私有 `audio-game-dispatch-original/`。

- 把指针表中存在的结构当作正常 dispatcher 可进入的定义，忽略 handler 的无符号比较与返回分支。
- 把 group2 初始化在假设绕过 guard 后的副作用，写成合法 AC～EF 已发生的行为；把 80 的相同 guard 情形忽略。
- 将已知 driver 范围的直接分支检索、或原 ROM 对齐字节 candidate 扫描，表述为排除了所有 computed／运行期绕过路径。
- 把 F0 当成普通编号 7A；忽略其独立 descriptor index 和独立 payload。
- 仅因一字节 PCM 或 F2-only score 没有普通音符，就宣称整个分派／优先级／初始化无副作用。
- 以既有 42 流未到达为依据，将额外音乐归为诊断；忽略已保存自然流程的 request／selected 证据。
- 只扫描音序原字节来计数控制 token，将参数、patch 或相邻数据误当作可达命令。
- 有限静态 walker遇到未知扩展、越界、零时长环或共享 loop／return 别名时仍声称全表覆盖。
- 将 header／token 静态可达、自然 selected、实际 chip 参数读取和扬声器输出混为同一证据级别。
- 把源字节、ROM／CPU 地址或任意程序位置放入语义审计表；这些内容只保留在 private provenance。
