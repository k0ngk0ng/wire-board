# 动画／形状目录扩展：验证范围

本文件在目录提取与新验证代码之前建立。目标为固定日版 ROM 的角色集合、姿态表及可确认动画帧，不把数据目录等同于角色玩法完成。

## 实现前失败场景

- 角色索引与初始化分派槽对应错误，或将未知中文名称猜成已确认映射。
- 只读初始化前的旧集合指针，未真正执行目标角色初始化。
- 用相邻非表字节、最小偏移或未经验证的猜测决定姿态表长。
- 相对表偏移基准错误，负偏移和共享姿态入口被误判。
- 动画帧的终止 bit7、记录边界、形状目录索引或可用 ROM 范围错误。
- 遇到未终止帧就强行截断，或用固定最大帧数当作自然表长。
- 同一数据键产生不同语义内容而被覆盖。
- 把解码再编码的自洽性当作独立原 CPU 验证。
- 不检查原 CPU 实际返回、对象／缓冲／声音副作用而声称新数据全部有效。
- 把原 ROM、模拟器或图片媒体混入正式语义数据目录。

## 验证步骤

1. 从已确认的初始化分派入口读取角色分支，并用原 CPU 初始化结果核对 `+50`、`+8c`、`+90` 集合／参数指针。
2. 用静态可达代码与显式边界证据建立可确认姿态范围。未确认范围记为未知，不延伸相邻字节。
3. 在已确认姿态中逐帧解码到显式终止记录；每个来源地址、字节摘要与范围保留。
4. 用独立原 CPU 调用 `0x1eb40` 的实际输出检查所提取的新帧，并使用已有独立 Go 动画语义作跨实现对照。任何未覆盖项目明确列出。
5. 交付只读语义数据、来源清单、覆盖／未知项及复跑脚本。原 ROM、原始内存与临时程序留在 `.local/`，不发布 OSS。

## 当前状态

已恢复 11 个来源角色集合的有界目录，共 4,732 条动画帧记录。原 CPU 对每条记录执行两种隔离场景，共 9,464 次调用；独立 Go 动画提交规则与全部结果一致，原生竞态和 Node/Wasm 验证通过。结果见 [Go 对照报告](../../../games/yuyuhakusho/data/animation-catalog-go-comparison.json) 和[验证记录](../../../games/yuyuhakusho/data/animation-catalog-validation.json)。

| 来源 ID | 帧记录数 |
| --- | ---: |
| 0 | 528 |
| 1 | 381 |
| 2 | 440 |
| 3 | 423 |
| 4 | 441 |
| 5 | 357 |
| 6 | 544 |
| 7 | 368 |
| 8 | 402 |
| 9 | 443 |
| 10 | 405 |

目录保留原初始化分派顺序，不猜测人物中文名。各集合的姿态表从 `+0x180` 开始存放帧记录；非零入口为姿态 0～190。第 191 项偏移为零，未作为动画解析。每个姿态在首个 `Flags.bit7` 记录处结束，末尾零填充单独计数。形状目录项与有效载荷均按原引用范围检查；最后一个角色集合只具备 ROM 末尾上界，其被引用的载荷仍须显式终止。原始地址、内容摘要、姿态范围及初始化结果保留在[提取清单](../../../games/yuyuhakusho/data/animation-catalog-manifest.json)。

初始化验证实际执行 `0x126c8`，分别选择 11 个来源 ID，并核对原 CPU 写入的动画集合、形状集合、移动参数、图形集合和图块基址。静态分派指令见 [初始化证据](static/animation-catalog-initializers.txt)。本轮静态分析使用直接 Ghidra；当前 REA 不支持 raw 68000，未把 Ghidra 结果标成 REA 反编译结果。

## 原 CPU 与独立规则对照

两种隔离场景分别使用朝向 0／3、动画计时 0／1、普通／边界坐标及空／已占用声音队列。四类形状缓冲的初始偏移在 0、240、256 之间轮换，覆盖起始、末槽与溢出复用。每次从同一研究存档恢复，并只设置已声明的测试输入；结果由原 CPU 运行至确认返回指令后记录。

Go 验证器由帧键自行选择只读数据，调用已有 `SubmitAnimationRecord`。每次比较完整 256 bytes 人物对象、1,024 bytes 形状缓冲、四个缓冲计数、待处理声音与四个声音队列字节，不屏蔽未知字节，不把原输出写回 Go 输入。9,464 次调用全部一致；这批关闭中断的隔离调用没有自然观察中那 8 次独立声音消费者干扰。

验证器测试先于实现编写，并保留缺实现失败记录。负向用例确认以下错误会被拒绝：帧时长改变、数据缺失或重复、人物／缓冲未知字节改变、计数或声音状态改变、语义输入与原字节不符、错误返回位置、帧键不匹配、场景缺失或重复、夹具未绑定目录摘要。全部调用完成后另检查只读库未被修改。

## 复跑

从此 worktree 根目录运行。固定 ROM、加钩子内核与观察器须已按[研究工具说明](../../../scripts/yuyuhakusho/README.md)准备；正式运行时不需要这些工具。

```sh
mkdir -p .local/md-yuyuhakusho/tmp
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/extract_animation_catalog.py \
  --out .local/md-yuyuhakusho/catalog-replay
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go run ./games/yuyuhakusho/cmd/verify-animation-catalog \
  -catalog .local/md-yuyuhakusho/catalog-replay/animation-catalog.json.gz \
  -oracle .local/md-yuyuhakusho/catalog-replay/animation-catalog-oracle.json.gz
```

使用已保留夹具验证原生／Wasm 规则：

```sh
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -json ./games/yuyuhakusho/cmd/verify-animation-catalog
GOOS=js GOARCH=wasm GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -json -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" \
  ./games/yuyuhakusho/cmd/verify-animation-catalog
```

## 覆盖边界

该目录覆盖已确认非零表项及其有界帧记录，尚未证明所有姿态都能通过正常玩法到达。它提供动画时序、图形参数和形状语义数据，不包含人物精灵、图块解压、渲染器、完整动作规则、电脑策略或完整对局。来源 ID 的存在也不代表对应人物已经可玩；目录与隔离调用数不能用于计算完整游戏的完成率。
