# GridSim 微电网仿真模块

> 版本: v2.5.2-dev | 模块: `internal/microgrid/` | 前端: `MicrogridEditor.vue`

## 功能概述

微电网仿真模块提供列表式设备管理和实时仿真能力，支持光伏、储能、负荷、充电桩四种设备类型，通过 IEC104 测点体系与标准规约无缝对接。

### 核心功能

| 功能 | 说明 |
|------|------|
| 设备管理 | 表单添加/编辑/删除设备，配置控制模式（远方/本地） |
| 拓扑展示 | SVG 拓扑图：方向感知流动效、自适应宽度、全设备展示 |
| 自动公式 | 引擎启动时自动生成 GRID 公式（Load+Charger+Battery−PV） |
| 实时仿真引擎 | 6-step tick 循环（PV→负荷→储能→功率平衡→store同步→公式求值） |
| 控制模式 | 远方(AO跟随) / 本地(策略驱动) |
| SVG 拓扑图 | 方向感知流动效、自适应宽度、全设备展示 |
| IEC104 测点映射 | 自动展开设备拓扑为标准测点表 |
| 点表导出 | 标准 xlsx 格式（兼容 enos 和 IEC104 实例） |
| 公式引擎 | `{测点名}` 引用 + 四则运算表达式解析 |
| 开关操作 | 实时联动 DI/DO 测点值（1=合/0=分） |
| 拓扑导入/导出 | JSON 格式备份和恢复 |

---

## 架构

```
web/MicrogridEditor.vue           ← 前端设备管理器 + SVG 拓扑
internal/microgrid/
  model.go                         ← 数据模型（Topology/Device/FormulaRule）
  engine.go                        ← 仿真引擎（tick/功率平衡/SOC/IOA索引）
  pointmap.go                      ← IEC104 测点展开
  handler.go                       ← REST API（6组端点）
  history.go                       ← 历史快照缓冲区
  formula_test.go                  ← 单元测试（21个）
pkg/library/store.go              ← 并发安全内存点表
internal/model/instance.go        ← 实例配置（PointsJSON持久化）
```

### 引擎执行流程

```
tick() 每一周期:
  1. PV 功率计算（AO setpoint / 保持当前值）
  2. 负荷/充电桩功率（保持 store 值）
  3. 储能功率（AO setpoint / 保持当前值）
  3.5 SOC 更新（ΔSOC = power × dt / capacity × 100）
  4. 功率平衡（GRID = Load + Charger + Battery - PV）
  5. 同步 store（_Power/_SOC/_SwStatus/_SwCtrl/_Status/GRID_P）
  5.5 公式评估（自动 GRID 公式 + 用户自定义公式）
  6. 快照记录
```

### 功率正负约定

| 量 | + 含义 | - 含义 |
|----|--------|--------|
| PV | 发电 | 无（始终 ≥0） |
| Battery | 充电 | 放电 |
| Load/Charger | 用电 | — |
| GRID_P | 从电网用电 | 向电网送电 |

---

## API 参考

### 端点列表

| 端点 | 方法 | 说明 |
|------|------|------|
| `/microrid/{id}/topology` | GET/PUT | 拓扑配置 |
| `/microgrid/{id}/device` | POST/PUT/DELETE | 设备增删改 |
| `/microgrid/{id}/control/{devId}` | POST | 开关控制 (?closed=true/false) |
| `/microgrid/{id}/dashboard` | GET | 实时数据（设备级功率数组） |
| `/microgrid/{id}/points` | GET | IEC104 测点列表（sorted + managed标记） |
| `/microgrid/{id}/formulas` | GET/POST/PUT/DELETE | 自定义公式 CRUD |
| `/microgrid/{id}/export-xlsx` | GET | 导出标准点表 xlsx |

### 仪表盘响应格式

```json
{
  "grid_power_kw": -35.2,
  "pv": [{"id": "pv1", "name": "PV-1", "power_kw": 48.5, "closed": true}],
  "battery": [{"id": "bat1", "name": "ESS-1", "power_kw": -30, "soc": 68, "closed": true}],
  "load": [{"id": "load1", "name": "L1", "power_kw": 32, "closed": true}],
  "charger": [...],
  "total_pv_kw": 48.5, "total_bat_kw": -30, "total_load_kw": 32, "total_charger_kw": 0,
  "battery_soc": 68
}
```

### 测点响应格式

```json
{"ioa": 101, "name": "PV-1_Power", "point_type": "AI", "value": 48.5, "unit": "kW", "managed": true}
```

`managed: true` 表示该测点由引擎管理，不可手动操作。

---

## IOA 分配规则

```
关口表:      AI 1~4, DI 1001~1002
设备 i (从0): AI 101+i×50+0..4, DI 1101+i×50+0..4, AO 3001+i×50, DO 4001+i×50
自定义测点:   在各设备段末尾递增
```

点表在拓扑保存时持久化到 `MicrogridConfig.PointsJSON`，引擎启动时加载。

---

## 公式语法

```
{测点名} 引用实时值     例: {PV-1_Power}
运算符: + - * / ( )    例: ({PV-1_Power} + {ESS-1_Power}) * 0.9
```

引擎自动生成 GRID 公式：
```
GRID_P = (Load_Power + Charger_Power + Bat_Power) - (PV_Power)
```

---

## 部署

```bash
tar xzf gridsim-v*.tar.gz
cd gridsim-v*/ && ./start.sh
# 浏览器 → http://localhost:8989
# 创建实例 → 选择微电网协议 → 进入拓扑编辑器
```

---

## 开发

```bash
go build ./...                    # 后端编译
go test ./internal/microgrid/     # 单元测试（21/21）
cd web && npm run build           # 前端构建
make dist                         # 三平台打包
```
