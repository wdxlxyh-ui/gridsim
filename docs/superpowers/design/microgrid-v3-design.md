# Microgrid v3 — 完整设计方案

> **日期**: 2026-05-23 | **状态**: 设计阶段
> **影响模块**: engine.go / model.go / pointmap.go / handler.go / MicrogridEditor.vue / api/index.ts

---

## 目录

- [1. SVG 拓扑图修复 + 主题优化](#1-svg-拓扑图修复--主题优化)
- [2. 仪表盘重构](#2-仪表盘重构)
- [3. 远方/本地控制模式 + 策略复用](#3-远方本地控制模式--策略复用)
- [4. 自定义测点 + 点表导出](#4-自定义测点--点表导出)
- [5. API 变更清单](#5-api-变更清单)
- [6. 兼容性分析](#6-兼容性分析)

---

## 1. SVG 拓扑图修复 + 主题优化

### 问题分析

当前 SVG 使用硬编码色值（`#fef0f0`, `#e8c560` 等），与 Element Plus 主题变量不统一。
设备渲染逻辑使用 `v-html="svgTopology"` 配合 computed，存在 TypeScript 类型问题导致部分设备未呈现。

### 修复方案

#### SVG 背景色

| 当前 | 替换为 |
|------|--------|
| `fill="url(#g)" opacity="0.4"` | `fill="var(--el-bg-color-page)"` |
| 固定网格 `pattern` | 移除（Element Plus 无背景网格） |
| 色值 `#e8eaef` | `var(--el-border-color-light)` |

#### 设备渲染

**根因**: `computed` 中 `dev.power` 为可选字段，部分条件判断短路导致未渲染。

**修复**: SVG computed 中统一使用 `(dev as any).power ?? 0` 或更安全的可选链。添加 `v-if="devices.length > 0"` 守卫，空设备时显示空态提示。

#### 母线延伸

当前 `minX/maxX` 逻辑正确，确保覆盖所有设备。

#### 电网/关口表样式

使用 `var(--el-color-primary-light-8)` 等 Element Plus 主题色替代硬编码。

---

## 2. 仪表盘重构

### 当前问题

| 字段 | 问题 | 原因 |
|------|------|------|
| `frequency_hz` | 不需要 | 用户明确不展示 |
| `total_generation_kw` | 聚合值，无设备级 | 无法对应具体设备 |
| `total_load_kw` | 同上 | |
| `battery_power_kw` | 聚合值，多电池时无效 | |
| `grid_power_kw` | 来源不清晰 | 应与 `GRID_P` 测点一致 |

### 新仪表盘结构

```json
// GET /api/v1/microgrid/{id}/dashboard
{
  "grid_power_kw": -35.2,
  "pv": [{"id": "pv1", "power_kw": 48.5}, {"id": "pv2", "power_kw": 0.0}],
  "battery": [{"id": "bat1", "power_kw": -30.0, "soc": 68.2}],
  "load": [{"id": "load1", "power_kw": 32.0}, {"id": "load2", "power_kw": 18.2}],
  "charger": [{"id": "ch1", "power_kw": 15.5}],
  "total_generation_kw": 78.5,
  "total_load_kw": 65.7
}
```

设备级功率直接从 `syncStoreLocked` 写入 store 的 `{dev.id}_Power` 测点读取，保证与 IEC104 送值一致。

### 前端仪表盘面板

```
┌──────────────────────────────────────┐
│ 并网点功率    PV总功率    负荷总功率   │
│ -35.2 kW      48.5 kW     65.7 kW   │
│                                      │
│ 光伏阵列 A     48.5 kW               │
│ ESS-1         -30.0 kW   SOC 68.2%   │
│ 车间负荷       32.0 kW               │
│ 充电桩 #1      15.5 kW               │
│ 办公楼         18.2 kW               │
└──────────────────────────────────────┘
```

---

## 3. 远方/本地控制模式 + 策略复用

### 数据模型扩展

```go
// ControlMode 控制模式
type ControlMode string
const (
    ModeRemote ControlMode = "remote" // 远方=跟随AO
    ModeLocal  ControlMode = "local"  // 本地=策略驱动
)

// StrategyConfig 自动变化策略配置（复用 detail 模块的 11 种策略）
type StrategyConfig struct {
    Type    string          `json:"type"`    // increment/random/csv/max/min/soc/energy/...
    Enabled bool            `json:"enabled"`
    Params  StrategyParams  `json:"params,omitempty"`
}

type StrategyParams struct {
    StartValue  float64 `json:"start_value,omitempty"`
    Step        float64 `json:"step,omitempty"`
    PeriodMS    int     `json:"period_ms,omitempty"`
    MaxValue    float64 `json:"max_value,omitempty"`
    MinValue    float64 `json:"min_value,omitempty"`
    // ... 全部11种策略参数
}

// Device 新增字段
type Device struct {
    // ... 现有字段
    ControlMode ControlMode    `json:"control_mode"`  // remote | local
    Strategy    *StrategyConfig `json:"strategy,omitempty"`  // local 模式下有效
}
```

### 引擎适配

#### 所有设备支持 AO Setpoint

**PV** (已实现 README 版本):
```
if ControlMode == ModeRemote:
    setpoint = readStoreValue("{dev.id}_Setpoint")
    if setpoint > 0: power = min(setpoint, RatedPowerKW)
    else: power = readStoreValue("{dev.id}_Power")  // 保持上次值
```

**Load/Charger** (新增):
- AO 端口: `{dev.id}_Setpoint` (当前无此字段)
- Remote: `power = readStoreValue("{dev.id}_Setpoint")` (上限 RatedKW)
- Local: `power = random * RatedKW` (现有逻辑) 或 策略驱动

**Battery** (已有):
- Remote: AO setpoint (现有逻辑)
- Local: 策略驱动或时段调度

#### 策略引擎集成

当 `ControlMode == ModeLocal` 且 `Strategy.Enabled == true` 时，在 tick() 中评估策略：

```
// tick() step X: 针对每个 local 设备
if dev.ControlMode == ModeLocal && dev.Strategy.Enabled:
    evaluateLocalStrategy(dev)
```

策略评估逻辑直接复用 `internal/detail/strategy.go` 中的 `evaluateStrategy` 函数。但需要将其改为可导出或复制策略评估逻辑到 microgrid 包。

**推荐方案**: 将 `strategy_eval.go` 放入 `internal/microgrid/` 包，复制 `detail/strategy.go` 中的 `evaluateStrategy` 相关代码，避免跨包依赖。

### 前端控制模式UI

在设备编辑弹窗中增加：

```
┌─ 设备参数 ──────────────────────────┐
│ 设备名称: [光伏阵列 A              ]  │
│ 控制模式: ● 远方(AO跟随) ○ 本地(策略) │
│                                       │
│ ┌─ 策略配置 (本地模式下显示) ──────┐   │
│ │ [递增] [随机] [CSV] [MAX] ...   │   │ ← 复用 DetailPage 的策略 Tab
│ │ 参数: ...                       │   │
│ └─────────────────────────────────┘   │
│ [取消]                      [保存]    │
└───────────────────────────────────────┘
```

### 测点页面策略复用

微电网的测点管理 Tab 改为类似 `DetailPage` 的布局：
- 每行增加「自动变化」配置按钮
- AI/AO 类型测点可配置策略
- 被微电网引擎管理的测点（`_Power`, `_SOC` 等）加上标签或置灰
- 普通测点（未映射到设备）保持原有可配置性

#### 测点管理 Tab 设计

```
┌─────────────────────────────────────────────────────────┐
│ 测点列表                                        [刷新]  │
│ ┌────┬────┬──────────────┬────────┬─────────┬────────┐  │
│ │IOA │类型│ 名称          │ 当前值  │ 自动变化 │ 操作  │  │
│ ├────┼────┼──────────────┼────────┼─────────┼────────┤  │
│ │ 1  │ AI │ GRID_P       │ -35.2  │ 引擎管理 ⚡│ —     │  │
│ │ 2  │ AI │ GRID_Q       │ -5.3   │ 引擎管理 ⚡│ —     │  │
│ │ 101│ AI │ pv1_Power    │ 48.5   │ 引擎管理 ⚡│ —     │  │
│ │ 102│ AI │ pv1_Setpoint │ 50.0   │ ─        │ [置数] │  │
│ │ 201│ AI │ load1_Power  │ 32.0   │ 引擎管理 ⚡│ —     │  │
│ │ 999│ AI │ 自定义测点    │ 0.0    │ ─        │ [配置] │  │
│ └────┴────┴──────────────┴────────┴─────────┴────────┘  │
│ AI 遥测 | DI 遥信 | PI 遥脉 | AO/DO 不可配置             │
│ ⚡ = 引擎管理 (不可手动操作)                             │
└─────────────────────────────────────────────────────────┘
```

**实现**: 后端 handler 中判断哪些 IOA 被引擎管理，返回 `managed: true` 标记。前端根据此标记禁用按钮。

---

## 4. 自定义测点 + 点表导出

### 自定义测点

当前 `Device` 已有 `CustomPoints []CustomPoint` 字段，但前端未实现编辑 UI。

**前端**: 在设备编辑弹窗中增加「自定义测点」区域：
```
┌─ 自定义测点 ─────────────────────────────┐
│ [+ 添加测点]                               │
│ ┌──────────┬──────────┬──────────┐       │
│ │ 测点名    │ 类型     │ 操作     │       │
│ ├──────────┼──────────┼──────────┤       │
│ │ Temp_Sensor│ AI     │ [删除]    │       │
│ │ Alarm_Light│ DI     │ [删除]    │       │
│ └──────────┴──────────┴──────────┘       │
└──────────────────────────────────────────┘
```

**后端**: `ExpandPoints()` 中为每个自定义测点分配 IOA（在设备 AI/Dx 段之后递增）。

### 点表导出

**新 API 端点**:
```
GET /api/v1/microgrid/{id}/export-xlsx
```

返回标准 IEC104 点表 `.xlsx` 文件。

**实现**:
1. 调用 `topology.ExpandPoints()` 获取完整测点列表
2. 使用 `excelize` 库创建 xlsx，格式为：
   ```
   | point-name | point-number | value-type | point-type | efficient | base-value | alias |
   ```
3. 文件名: `{实例名称}_point.xlsx`
4. 前端: 拓扑 Tab 增加「导出点表」按钮

**兼容性**: 导出的 xlsx 可直接用于 `eno` 系统或作为标准实例的 `point.xlsx` 加载。

---

## 5. API 变更清单

| 端点 | 方法 | 变更 |
|------|------|------|
| `/microgrid/{id}/dashboard` | GET | 返回结构变更：移除 freq，增加设备级 power |
| `/microgrid/{id}/points` | GET | 返回新增 `managed: bool` 字段 |
| `/microgrid/{id}/export-xlsx` | GET | **新增** — 导出标准点表 xlsx |
| `/microgrid/{id}/topology` | PUT | Device 结构新增 `control_mode` / `strategy` 字段（向后兼容） |

---

## 6. 兼容性分析

| 变更 | 影响 | 兼容措施 |
|------|------|---------|
| Device 新增 `control_mode` | 现有拓扑 JSON 无此字段 | Go 默认 "" → 引擎视为 `remote` |
| Device 新增 `strategy` | 现有拓扑 JSON 无此字段 | `omitempty`, nil 值不参与评估 |
| Dashboard 移除 `frequency_hz` | 前端可能引用 | 前端同步更新，无其他消费者 |
| Dashboard 增加设备级 power | 新字段，只增不减 | 完全向前兼容 |
| 测点 `managed` 标记 | 新字段 | 前端可选使用，不使用的实例忽略 |
| `export-xlsx` | 新端点 | 无兼容问题 |
| 引擎 tick() 新增策略评估 | local 模式设备 | remote 模式设备行为不变 |

### 迁移路径

1. 部署新后端 → 现有拓扑（无 control_mode）默认为 remote 行为
2. 前端更新后 → 用户可逐个设备配置 control_mode + strategy
3. 策略引擎仅评估显式配置了 `local + strategy` 的设备
4. 导出按钮为纯新功能，不影响现有流程

---

## 实施建议

推荐分两阶段实施：

**Phase A (高优先级)**:
- SVG 主题修复 + 设备渲染修复
- 仪表盘重构（移除 freq，加设备级 power）
- 测点 `managed` 标记
- 自定义测点 UI + 后端

**Phase B (低优先级)**:
- 控制模式 + 策略引擎
- 点表导出
- 测点页面策略复用
