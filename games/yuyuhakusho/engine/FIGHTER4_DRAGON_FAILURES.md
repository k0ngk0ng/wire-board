# 飞影龙与强化失败场景（实现前冻结）

范围：原 CPU `13BB6` 龙头、`13E9A` 身体段、`13FC8` 强化子对象。静态入口和交接不是执行覆盖；以下均须由同一 ROM/core 的真实回调补证。

- 初始化必须先于 pause/slow；仅清原图形偏移、参数和四 cache；保留未写的 clock index/flags/completed、低 word、脏字段。头与强化写 HP255，身体不写 HP/AN/shape。
- pause 的符号位和任意非零 slow 直接请求一次绘制，不能推进时钟、声音、角度、历史、FX 或父状态；初始化仍生效。
- 龙头声音计数旧零才写 F0；计数只改 Energy 高 word，255 回零，低 word 保留。时钟真实循环、终止标记和脏 flags/completed 不得导致擅自删除。
- 波相位只改 Gravity 高 word，先加32再 mask1023；signed WORD 曲线不是旧 signed byte 表。有效角为波值左移7加完整 Direction，保留低 word、带符号前缀和未 mask 的查表地址；不得 wrap。
- 16 个 Held 方向（忽略高 nibble）、方向边界0/511/512/513/1023与完整低 word，严格按 signed word 比较选择加减16后 mask。
- 头 Facing 仅改 bit0，身体 Attributes 仅改高 byte bit3/4；pose/parameter 8 桶边界和非0/1 Facing 不可归一化。
- 位置全 long 加减，包含小数进位/借位和 signed word边界。age 只改高 word，signed >=2048 才回1024；负 age、32767/32768/65535及31/32不能被 unsigned 规则误写。
- 四 bank、32槽六 plane 逐字段核对；头只覆当前槽，Layer+Skip 写完整 word、Status只写高 byte；原空洞及低 Status byte保留。身体仅读 Layer byte，X/Y/Angle仅写高 word，旧小数/低 word 保留，完全不读 Owner。
- age signed <=31 且低两位为0时独立分配身体段，包括负 age；原 FX 先增 cursor、最多15槽，当前空槽不能被扫描，分配失败仍推进游标，脏槽除原写字段全保留。
- 龙头先写历史/分配段，再判地面、上界、左右64及 Contact bit3/4；signed 比较和 word边界回绕一致。地面/Contact写 Status2并分配1D冲击；上界/左右越界写Status1且无冲击；优先级不得改变。
- 身体每call独立更新 age&31，从对应 bank读样本；Status1/2均删除且无Draw，其他任意Status可绘制；不能加Owner校验、重置bank或从最终对象重建历史Draw。
- 3556按16字节块清前缀，size0默认64；各16-byte边界、非16整倍数和非法越界需保留原清除语义，错误必须原子回滚。
- 强化复制父X/Y高 word、完整 Facing/Depth、Layer+4 byte回绕；子小数保留，父Aux64.bit4/AttackValue左移byte回绕独立生效。
- 父HP负或pose0F/10跳过消耗；其他按Energy高 word signed >=220选完整long减40000，否则减5000，含低word借位。跳过消耗后仍要判断Energy高 word负。
- 负能量路径原字段逐个构造，包括完整Energy/MovementSource、StatusDelay、Aux7E、StunTicks、action/pose/Gravity/VY/VX、EnabledStages/ActionFlags、clock。Facing任何非零取正VX；未写字段保留，不能调用宽ResetActionState。
- AN缺失、坏shape、曲线缺失/范围外、bank未对齐、FX cursor未对齐和晚期清除错误不能泄漏child、parent、history、pool、sound或shape buffer变化。
- 保存回调当时每次1580的完整WorldDraw值和全局序号；独立FX地址顺序、重用和清除不得修改此前Draw。真实单初态head→多段→history读取→消失全链不得逐步回填CPU答案或clock。
- 原core唯一ordinary3602control保持逐帧RAM/video完全相等。每回调比较全父三子、全部16FX、六plane完整typed history、声音/RNG/shape，并审计全64KiB实写；规则外变化逐byte串联真实IRQ delta，队列消费者单独记录，不可用宽排除掩盖。

本清单先于新增类型、capture、测试及行为实现。类型与函数测试的冻结SHA、真实own-symbol red和最终native/Wasm产物记录在对应verification文件。
