# 河流海图存档的跨架构坐标兼容

2026-10-11。GitHub Actions 的分片 13 在 `TestCatanTwoRiversSeaDeadEnd` 读取冻结局样本时失败，错误为「河流海图地形数字或尺寸不符」。相同样本在本地原生 ARM64 通过；改用本机 amd64 运行后复现相同错误。

## 原因与修复

地图生成计算在两种架构上有浮点尾数差异。例如第 9 块地形的 X 坐标，存档值为 `453.37059831359926`，amd64 生成值为 `453.3705983135992`，差约 `5.68e-14`。原校验用 `reflect.DeepEqual` 比较整块地图，因而将显示坐标的舍入差异误判为无效存档；电脑动作检查继而也无法通过。

按用户要求，河流地图的地块和交点坐标先乘以 `1_000_000`，统一四舍五入并转成 `int64`，之后只比较整数。没有浮点差值、误差阈值或浮点相等判断。地图生成时的交点标识精度是 `0.001` 单位，比较用的整数格更细；不改变标识、地图生成或已有存档值。

地形、骰点、地块编号、交点编号、港口标记、边和地块的连接关系仍精确比较。NaN、无穷值和超出精确整数表示范围的值在转换前拒绝，明显坐标位移也仍拒绝。修复覆盖河流的新海岸、迷雾、沙漠、遗忘部落、新世界及已有五六人海图校验，不改变基础版规则。

## 验证与复跑

先新增失败场景：9 类地图的坐标发生单 ULP 舍入变化、15 类地形或拓扑损坏、验证不得修改存档，以及冻结局样本的字段差异诊断。改为整数比较前另加入 11 项检查，覆盖正负坐标的同格／相邻格、带符号零、非有限值及转换溢出。变更前复现失败后才修改生产校验。

以下定向测试分别在本机 ARM64 和 amd64 通过，包含原 CI 失败的两个子测试及既有冻结判断用例：

```sh
source scripts/env.sh
go test ./internal/game -run '^TestCatanGeometryIntegerGrid$|^TestCatanRiversGeometry|^TestCatanRiversFrozenGeometryDiagnostic$|^TestCatanTwoRiversSeaDeadEnd$' -count=1 -v
# 需要主机具备 amd64 执行能力；本机已验证可运行。
GOARCH=amd64 go test ./internal/game -run '^TestCatanGeometryIntegerGrid$|^TestCatanRiversGeometry|^TestCatanRiversFrozenGeometryDiagnostic$|^TestCatanTwoRiversSeaDeadEnd$' -count=1 -v
```

其他河流地图、配方、拒绝非法状态、恢复及双人海图自然完整局的定向回归通过（34.198 秒），`go vet ./...` 通过，结果保存在下述日志。没有为了复现这个错误重新运行全部分片。

产物保留在 `.local/rivers-amd64-before.log`、`.local/rivers-geometry-diagnostic.log`、`.local/rivers-geometry-before.log`、`.local/rivers-integer-before.log`、`.local/rivers-integer-arm64-after.log`、`.local/rivers-integer-amd64-after.log` 和 `.local/rivers-integer-regression.log`。GitHub 上整轮结果以修复提交对应的运行状态为准，不能把本地通过写成 CI 已通过。
