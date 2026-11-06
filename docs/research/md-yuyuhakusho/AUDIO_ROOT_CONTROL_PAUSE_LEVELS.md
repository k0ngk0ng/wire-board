# Root 暂停恢复载荷与状态边界

原 mixed music／primary 向量确认：暂停捕获点的请求 cache 可见低 7 位与芯片 24 个有效 FM TL 全等；静音后 cache 保留旧值而当前芯片 TL 全为 127；恢复后逐项回到捕获前的值。Root 因此只保存暂停期间需要的 24-byte `PauseLevels`，不保存整个 YM、18 voices 或常驻硬件镜像。

## 原观察

原向量从 quiet cleared world 发布 `82`，第 26 步发布 `A7` 形成声明的 mixed/claimed 压力状态。启用控制器暂停资格后，以真实 Start 边沿暂停／恢复：第 31 至 69 步为 `FF`，第 71 步恢复。这个 A7 是完整 primary 目录中的可调用压力输入，不在已审 fight139 producer 集合；不能据此扩大比赛可请求 ID。

| 点 | 原结果 |
| --- | --- |
| pause capture／保存 cache 前 | cache 可见低 7 位 = native chip 24 TL。 |
| FM mute 后 | 当前硬件 24 TL = 127，cache 仍保留 pre-mute 值。 |
| resume 写前 | 芯片仍 muted，cache 仍有恢复信息。 |
| resume 结束 | 当前芯片 TL 与 pre-mute 全等，且再次等于 cache 可见值。 |

原数据、两个 24-request 顺序、源 SHA 见 [pause-levels-original.json](../../../games/yuyuhakusho/verification/root-control/pause-levels-original.json)。更宽真实 fight 向量将以独立 supplement 记录，不覆盖本压力证明。

初稿 pause fixture 只写 bit40，但 quiet world 没有 pause-enabled bit0，原 `ToggleWorldPause` 正确清掉了 bit40，实际没有暂停。新的生命周期检查先保存这个真实 red；随后设置明确 enabled controller profile，并用实际 Start press/release，未改 core／原规则或降低预期。旧源码、失败数据和修正记录均保留。

## 纯载荷 API

`rootcontrol.CapturePauseLevels(chips.YMState)` 只从唯一 chip 的临时 view 拷贝 6×4 个 logical-operator level。它拒绝超出 7-bit 可见域的值。`PauseMuteWrites` 返回原 24 个 mute 请求，`PauseRestoreWrites` 从该 pending payload 返回原恢复请求；二者不自行运行虚拟 tick、不发布 PCM stop，也不重置 voice 参数。

请求顺序保持 channel 2、1、0，每个 channel 的 physical operator 顺序 0、1、2、3，每笔 bank0 后 bank1。chip 的 logical operator 顺序为 0、2、1、3，portable 原向量明确完成该映射；不把数组下标相同当作无需转换。原请求的硬件忽略 bit7 只用于研究 provenance，pending 只持可见 low7，恢复输出按已证的有效字段比较。

生命周期由完整 Root coordinator 管理：只有实际 pause capture 后才能设置可选 pending；mute、恢复、清除必须与全部 detached owners 一起成功提交。resume 或 `04` 成功后释放。helpers 本身不持长期指针、phase、pause byte、寄存器或其他 owner。

## 最小 Root state

`rootcontrol.State` 只持 `Priority`、`Selected`、`ChoiceBits`、一份嵌入的 `FadeState` 和可选 `PauseRestore`。新状态的 selected 为 `FF`。`RestoreState` 校验已审 fight choice bits 与 pending 值，并深拷贝恢复载荷；不把 caller 的可写数组变成另一份共享 owner。

World queue、World `AudioPause`、Scheduler、score tempo／voices、PCM 和 chips 都不在此结构内。manual control／duck／delta／signal／FM3 在已审闭包中派生为零，不持久保存这些恒零字段。外层还需结合实际 pause byte 与 coordinator phase 校验 pending 生命周期，不能仅凭一个 state 构造器完成全流程恢复。

## 验证范围

pause helpers 的四项测试与 Root state 的三项小型验证均先取得真实缺 API overlay red，冻结后才实现。原生 race、Wasm 与 vet 通过。原载荷通过 JSON 保存／恢复，真实 `chips.Interactive` 按原 mute／restore 请求恢复可见 TL；另验证两个不同 pre-level 状态 mute 后无法从当前 chip 唯一恢复原信息，pending 是有独立语义的信息。

本层只证明载荷、原写序和独立状态边界。它不支付 24 次 YM 服务的完整成本、不插入 IRQ／Z80 steal，也不表示 pause、Root fullclear 或完整 driver coordinator 已经完成。全部命令与来源由各专属 validation 绑定；没有修改 frozen leaf、score、PCM、chips 或平台。
