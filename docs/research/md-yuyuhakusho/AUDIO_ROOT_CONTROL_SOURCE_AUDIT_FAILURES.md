# Root control 来源审计的先行失败场景

本轮仅写入新的 `AUDIO_ROOT_CONTROL_SOURCE_AUDIT*` 文档、检查器和结果，不执行原 CPU、不加载 native core、不安装工具，不修改冻结 leaf、score、engine 或总索引。

- 将 139 个比赛候选、11 首比赛音乐、12 个 primary 与全 ROM 或原菜单的可达性混为一谈；把未发现 E2／FF 解释为全游戏不可能执行。
- 只扫描 catalog 的已转换事件，遗漏原数据中被删除的 command；或将 argument／encoded payload 中的 E2、FF 当作语法命令。
- 搜索 F8xx 字节后直接将命中解释为指令；未区分已保存的 Ghidra 指令、由明确分派表链接的原字节和未分类数据候选。
- 没有恢复 FF 子分派表到 `393c4/393ec` 的联系，误称 F828/829/A 只有当前 header 内 producer；或将该代码存在直接称为比赛／菜单实际执行。
- 将 F805 命名为 fade period，仅因位置相邻而新增状态；遗漏 01／05 的初始 remaining 不同及后续直接置常量 3。
- 将唯一带 ordinary descriptor flag5 的 7F 归入当前 139 个比赛候选；忽略 descriptor 身份、仲裁接受条件与请求／实际 duck 执行之间的区别。
- 将 04 的 232 个 long clear 与 music-reset 的 136 个 long clear 混用；遗漏 DBF 的初始值加一、末地址或 primary／secondary 保护边界。
- 将 music-reset 的 priority save／restore 和 selected=FF 合并为一次无序 reset；误给该 leaf 添加完整 keyoff／PSG mute。
- 忽略 bulk clear 同一 long 跨越不同 owner 字段，或把全部 Root／score 清理延迟到 tick 末；误清范围外的 Scheduler、Z80 PCM 或 FM TL cache。
- 将静态展开的 keyoff／TL mute 顺序写成已观察执行或 master-cycle 证明。

检查器先以缺失审计产物产生实际 red，再实现只读审计器。最终产物绑定固定 ROM、已有列表、grammar helper、旧证明与自身来源 SHA；所有否定结论都保留搜索域和未排除项。
