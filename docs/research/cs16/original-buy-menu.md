# 所选 CS 1.6 服务端样本的购买菜单

本次从已保存的 `cs.so` 文件独立恢复菜单分支、武器 ID、实体名指针与实际购买调用，再与固定提交的 ReGameDLL_CS 对照。普通地图下，各类别的槽位顺序一致。没有使用网上 CS 1.5 快捷键表作为原版依据。

样本来自[固定的第三方归档提交](https://github.com/martinsam16/cstrike/blob/651d0d248ba04028b1b8167b156888b522bbcb73/opt/hlds/cstrike/dlls/cs.so)，本地路径为 `.local/cs16/research/original/cstrike/dlls/cs.so`，大小 11,688,859 字节，SHA-256 为 `608904a0357af349466c7b34e69f224268090e3b553b5d46bf13b180001c9d8c`。这是 ELF32 i386 文件，保留符号与 DWARF；尚未取得发行方签名或官方摘要，也没有运行该文件。

REA 当前会话没有可用的深度原生 provider：Ghidra、Hopper 的已配置文件不存在，IDA 未配置。本次保存了 REA 会话诊断，随后使用机器已有的 Apple LLVM 21.0.0 `objdump`、`dwarfdump` 进行静态读取。下述反汇编与 DWARF 结果属于普通工具证据，不冒称 REA 反编译结果。没有安装工具、改动配置或打开原版游戏进程。

## 普通地图的槽位

表中编号保留原菜单槽位。记号 `B31` 表示打开购买菜单、选择类别 3、选择子项 1；客户端把 B 绑定到购买命令的行为不属于这次服务端文件检查。

| 类别 | CT 槽位顺序 | T 槽位顺序 |
| --- | --- | --- |
| 1 手枪 | 1 Glock 18、2 USP、3 P228、4 Desert Eagle、5 Five-Seven | 1 Glock 18、2 USP、3 P228、4 Desert Eagle、5 Dual Elites |
| 2 霰弹枪 | 1 M3、2 XM1014 | 1 M3、2 XM1014 |
| 3 冲锋枪 | 1 TMP、2 MP5、3 UMP45、4 P90 | 1 MAC-10、2 MP5、3 UMP45、4 P90 |
| 4 步枪与狙击枪 | 1 FAMAS、2 Scout、3 M4A1、4 AUG、5 SG550、6 AWP | 1 Galil、2 AK47、3 Scout、4 SG552、5 AWP、6 G3SG1 |
| 5 机枪 | 1 M249 | 1 M249 |
| 6 主武器补弹 | `BuyAmmo` 的武器槽位 1 | 同 CT |
| 7 副武器补弹 | `BuyAmmo` 的武器槽位 2 | 同 CT |
| 8 装备 | 1 防弹衣、2 防弹衣与头盔、3 闪光弹、4 HE、5 烟雾弹、6 夜视仪、7 拆弹器、8 盾牌 | 1 至 6，与 CT 对应项相同 |

因此普通购买菜单的 `B31` 是 CT 的 TMP 或 T 的 MAC-10，`B32` 才是 MP5。步枪第一项是 FAMAS／Galil。装备中的拆弹器使用原槽位 7，不能因为夜视仪或其他项不可用而重新编号。

购买成功仍受 `CanPlayerBuy`、`CanBuyThis`、资金、现有装备和地图条件约束。拆弹器另要求 CT 且地图有爆破目标。CT 非爆破图的装备菜单掩码为 `0x2bf`，隐藏 7 而保留 8；爆破图为 `0x2ff`；T 为 `0x23f`，仅包含 1 至 6 及退出。VIP／AS 模式另有禁用项，本表不把普通地图顺序扩写为所有特殊模式均可购买。

## 从分支到购买行为的证据

`objdump -d` 中部分调用显示为本指令附近地址，这是 ELF 动态重定位尚未应用的编码。实际被调函数由 `.rel.dyn` 中的 `R_386_PC32` 确定，不能把这些显示地址当成函数自调用。

| 路径 | 静态证据 |
| --- | --- |
| 队伍与菜单状态 | DWARF 明确 `m_iTeam` 在 `0x1dc`，`TERRORIST=1`、`CT=2`；`m_iMenu` 在 `0x348`，购买菜单为 4，冲锋枪子菜单为 9 |
| 主菜单分派 | `ClientCommand` 的菜单状态表在 `0x1d51ec`；购买类别表在 `0x1d5224`，第 3 项指向 `0xc588b`，打开冲锋枪菜单并写入菜单状态 9 |
| 冲锋枪 | `BuySubMachineGun` 位于 `0xbbfd0`；槽位 1 在 `0xbc043` 比较队伍，选择 TMP 的 ID 23／MAC-10 的 ID 7；槽位 2、3、4 分别选择 ID 19、12、30。所选实体名进入统一调用点，重定位 `0xbc1b5` 明确指向 `CBasePlayer::GiveNamedItem` |
| 手枪 | `BuyPistol` 位于 `0xbbb70`；槽位 5 根据队伍选择 ID 11／10。重定位 `0xbbd05` 指向 `GiveNamedItem` |
| 霰弹枪 | `BuyShotgun` 位于 `0xbbe20`；`0x1d5244` 的 ID 表为 21、5，名称指针表位于 `0x1d5254`。重定位 `0xbbf73` 指向 `GiveNamedItem` |
| 步枪 | `BuyRifle` 位于 `0xbc450`；`0x1d5108` 的槽位跳表将 1 至 6 分别分派到队伍条件选择块，最终所选实体名经重定位 `0xbc731` 调用 `GiveNamedItem` |
| 机枪 | `BuyMachineGun` 位于 `0xbc790`，只接受槽位 1，检查 ID 20；实体为 `weapon_m249`，重定位 `0xbc8c5` 指向 `GiveNamedItem` |
| 装备 | `BuyItem` 包装函数位于 `0xbc930`，CT 允许 1 至 8、T 允许 1 至 6；实际实现位于 `0xbb340`，槽位表位于 `0x1d50e4` |
| 装备效果 | 衣、衣盔和三种投掷物进入 `GiveNamedItem`；夜视仪在 `0xbb806` 写入 DWARF 确认的 `m_bHasNightVision`；拆弹器在 `0xbb39e` 写入 `m_bHasDefuser`；盾牌经重定位 `0xbb955` 调用 `GiveShield`。相应正常路径另有扣款调用 |

ReGameDLL 对照使用[固定提交的 client.cpp](https://github.com/rehlds/ReGameDLL_CS/blob/1f16a7f25c14225931f8df1abc51f68d18ded839/regamedll/dlls/client.cpp#L1177)和[装备槽位定义](https://github.com/rehlds/ReGameDLL_CS/blob/1f16a7f25c14225931f8df1abc51f68d18ded839/regamedll/dlls/client.h#L45)。对照结论限于上述有限菜单映射；参考实现的购买包装与所选文件中的内联分支结构不同，不能据此宣称整个游戏实现等价。

## 复跑与产物

完整地址、实体指针、槽位矩阵、符号重定位、DWARF 枚举和文件摘要保存在 `.local/cs16/research/hud/menu-evidence.json`；短结论保存在 `.local/cs16/verification/hud/menu-summary.json`。未修改游戏、Go 规则、客户端、模型或光照。

```sh
/usr/bin/objdump -t .local/cs16/research/original/cstrike/dlls/cs.so
/usr/bin/objdump -d --demangle --start-address=0xbbfd0 --stop-address=0xbc236 .local/cs16/research/original/cstrike/dlls/cs.so
/usr/bin/objdump -R .local/cs16/research/original/cstrike/dlls/cs.so
/usr/bin/objdump -s -j .rodata --start-address=0x1d50e4 --stop-address=0x1d525c .local/cs16/research/original/cstrike/dlls/cs.so
/usr/bin/dwarfdump --show-children --name=TeamName --name=WeaponIdType --name=_Menu .local/cs16/research/original/cstrike/dlls/cs.so
```
