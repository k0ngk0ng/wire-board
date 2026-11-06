# 乐器查找与分阶段 FM 传送验收

新增 `BeginInstrumentTransfer`／`WriteNext`／`Finish` 已连接具体乐器的查找工作、完整参数写入 caller、level 更新及实际 canonical YM 服务。阶段接口在每条预期声音意图到达时执行对应访问，不提前发完一批寄存器。便捷 `ServeInstrument` 复用同一实现，结果与分阶段操作相同。

原程序在既有两份固定 core 上完成 220 个声明私有场景、880 次执行。无 recorder／有 recorder 的完整 native 状态与 RAM 一致，两 core 的 128,106 个保留指令边界、访问与请求一致，包含：

- 八种算法、六种 FM channel、music／primary／secondary 的选择路径、claimed early return、负 base、carrier 及 carry 抑制。
- 同音色的不同选择引用，以及 74 个记录的较长查找；未按音色 ID 合并成本。
- 4,082 个 canonical YM 请求、8,164 次 Main 端口写入和 36,738 次规范化 Main 访问。

Go 测试在实现前冻结，真实缺 API 的 red 保留。实际 mixed Runtime 自行产生 bus／mailbox／YM Busy 反馈，与全部原观察 Main 时刻、访问顺序、有效数据和阶段最终 clock 一致；不是用原反馈回放。raw oracle 保留硬件忽略位，Go 使用已有 typed chip-effective 值，没有伪称 raw byte 全等。

原生竞态与实际 Node/Wasm 各通过 41 项顶层测试、2,307 条记录，无跳过，含所有旧声音时序回归；静态检查通过。错误意图、提前／重复 Finish、失败后重入、共享操作预算与 detached 分支处理通过。期望切片不能修改内部视图，分阶段／完整操作的实际 mixed 状态及访问一致。

源码、oracle 和可重复命令见 [validation.json](validation.json)，原生来源与数量见 [native-evidence.json](native-evidence.json)。离线审计为 `scripts/yuyuhakusho/verify_instrument_clock.py --audit`；复跑用 `--mode native|wasm|vet --out <新的仓库内目录>`。本次 recorder 临时二进制、空目录和重复 oracle 已清理，原 core／ROM、唯一验收夹具与旧 goldens 保留。

load 的结束边界在 pan-routing 尾部之前；level 包含自身 return。额外 IRQ、完整世界／声音 producer、其它现场参数、pan／DAC 重定向及平台播放仍由后续实际 owner 完成。本批没有完成整场逐采样音频或切换平台试玩构建。
