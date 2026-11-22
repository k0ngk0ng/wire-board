# 空仓自动装填与手枪编号

所选原服务端样本为 `.local/cs16/research/original/cstrike/dlls/cs.so`，SHA-256 为 `608904a0357af349466c7b34e69f224268090e3b553b5d46bf13b180001c9d8c`，大小 11,688,859 字节。来源、身份限制与完整菜单见 [购买菜单记录](original-buy-menu.md)。本次只用现有 Apple LLVM 工具与 Python 读取 ELF32 静态字节、符号和动态重定位，没有执行原版，也没有将 ReGameDLL 当作原版运行证据。

## 固定样本证据

`BuyPistol` 位于 `0xbbb70`。`0xbbbc8` 的槽位 3 条件跳至 `0xbbbf8`，加载武器 ID 1 及 `0x1d36c1` 的 `weapon_p228`；`0xbbd70` 的槽位 4 条件跳至 `0xbbdc0`，加载 ID 26 及 `0x1d36cd` 的 `weapon_deagle`。这两个名称最终进入 `0xbbd05` 的 `GiveNamedItem` 动态重定位。因此保留 B13=P228、B14=沙鹰。

`CBasePlayerWeapon::ItemPostFrame` 位于 `0x1351e0`。`0x1354b0` 对攻击按钮掩码 `0x801` 检查，持有任一攻击键就跳过空闲装填分支。`0x13576d` 检查空弹匣，`0x135795..0x1357a3` 要求主攻击剩余冷却严格小于零，`0x1357a9..0x1357d0` 要求两种三连发计时器均为零。`0x1357dc` 最后取得虚表偏移 `0x164` 的 `Reload`。

两把散弹枪有额外入口：M3 的 `0x15127f` 空仓分支跳至 `0x151560`，在 `0x151565` 调用虚表偏移 `0x164`；XM1014 的 `0x15e53f` 跳至 `0x15e830`，在 `0x15e835` 调用同一偏移。ELF `.rel.dyn` 将各武器虚表的该位置分别绑定至 `CM3::Reload` 和 `CXM1014::Reload`。这支持它们在按住主攻击时启动空仓装填；完全浸水的先行禁射分支不会进入该入口。

参考重实现另使用 ReGameDLL_CS 固定提交 `1f16a7f25c14225931f8df1abc51f68d18ded839` 的 `weapons.cpp::ItemPostFrame`、`wpn_m3.cpp::PrimaryAttack` 和 `wpn_xm1014.cpp::PrimaryAttack` 核对条件。参考源码的修复分支与所选样本有区别；本次不扩大为所有原版版本等价的结论。

## 实现范围与验证

普通枪空仓有备弹、左右攻击均松开、三连发结束且冷却严格结束后开始装填。M3、XM1014 在 live 且未完全浸水时，按住主攻击也可启动空仓装填。仍复用现有 R 装填、逐颗填装、换枪取消和装填事件，不新增网络字段或每帧提示事件。冷却用 100 Hz 整数权威帧，原样本浮点计时与现有散弹装填阶段仍存在已登记的量化差异，本次没有重写全部装填动画时钟。

失败场景先列于 [auto_reload_failures.md](../../../games/cs16/engine/auto_reload_failures.md)，规则 E2E 位于 `auto_reload_e2e_test.go`，通过真实 `QueueInput`／`Advance` 验证全部 24 枪、持续攻击与松手、严格冷却边界、备弹、散弹中止、三连发、换枪、死亡、准备期和恢复。

原始反汇编、可复跑 ELF 读取脚本及摘要位于 `.local/cs16/research/auto-reload/`；RED、GREEN、race 和验证摘要位于 `.local/cs16/verification/auto-reload/`。本层验收不代表浏览器动画、声音或 Linux 原版进程运行验收。
