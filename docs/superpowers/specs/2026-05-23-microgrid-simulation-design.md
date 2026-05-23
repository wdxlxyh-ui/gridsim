# 微电网仿真模拟器设计文档

> GridSim 子系统 — 嵌入式微电网仿真模块
> 日期：2026-05-23 | 版本：v2.0 | 状态：设计稿

---

## 1. 概述

### 1.1 目标

在 GridSim 平台内嵌入微电网仿真能力，使得一个 GridSim 实例可以运行完整的微电网动态仿真——包含光伏、储能、充电桩、负荷等元件的电气建模与逻辑控制，并通过 IEC 104 协议对外暴露所有电气测点，供 SCADA 或第三方系统接入测试。

### 1.2 设计原则

- **嵌入式复用** — 微电网模块作为 `internal/microgrid/` 包嵌入，不做独立进程
- **V1.0 仅 IEC104** — 当前版本只支持 IEC104 协议对外暴露测点
- **实例即拓扑** — 一个微电网实例对应一套完整拓扑，在原有实例管理中统一管理
- **先创建后配置** — 创建微电网实例时仅需端口号，拓扑和参数在详情页配置完成后方可启动
- **单母线单关口** — 每个微电网有且仅有 1 条母线和 1 个关口表，所有设备通过开关接入母线
- **灵活组合** — 设备类型和数量可任意组合，不要求包含所有类型（如可只有储能+关口表、或光伏+储能+负荷等）
- **开关影响功率** — 每个设备通过专用开关接入母线，开关分/合直接影响功率计算结果
- **点表自动展开** — 拓扑完成后自动生成 IEC104 标准点表，并支持导出

### 1.3 与现有架构的关系

```
┌───────────────────────────────────────────────────────────────┐
│                    GridSim (serve 模式)                         │
│                                                                │
│  ┌───────────────────┐  ┌───────────────────┐                  │
│  │  IEC104 Instance   │  │  微电网 Instance   │                  │
│  │  (Protocol+Engine) │  │ (new)              │                  │
│  └────────┬──────────┘  │  - MicrogridEngine  │                  │
│           │             │  - IEC104 Protocol  │                  │
│  ┌────────▼─────────────▼────────────────────┐                  │
│  │              Manager (统一实例管理)            │                  │
│  │  - Start / Stop / Restart                 │                  │
│  │  - GetStore / GetEngine / GetMicrogrid     │                  │
│  │  - 上限 1000 实例                          │                  │
│  └────────▲─────────────────────────────────┘                  │
│           │                                                     │
│  共享基础设施:                                                   │
│  - library.Store ↦ 测点内存存储                                  │
│  - detail.Engine ↦ 自动变化策略引擎                               │
│  - HTTP API + MCP ↦ 统一对外接口                                 │
│  - Web UI (Vue 3) ↦ ConfigPage / MonitorPage / DetailPage       │
└─────────────────────────────────────────────────────────────────┘
```

---

## 2. 微电网拓扑模型

### 2.1 拓扑架构

微电网采用**单进线单母线**拓扑：一个关口表（并网电表）上接大电网、下接一条母线；所有设备通过专用开关接入该母线。

```
                    电网 (Grid — 外部大电网)
                        │
                    ┌───┴───┐
                    │ 关口表  │  (Grid Meter — 唯一 PCC)
                    └───┬───┘
                        │
                    ┌───┴──────────────────────────────┐
                    │  母线 (10kV Bus — 唯一)             │
                    └───┬──────────────────────────────┘
         ┌──────────────┼──────────────┬──────────────┐
         │              │              │              │
     ┌───┴───┐      ┌───┴───┐      ┌───┴───┐      ┌───┴───┐
     │ QF1   │      │ QF2   │      │ QF3   │      │ QF4   │
     │ 开关   │      │ 开关   │      │ 开关   │      │ 开关   │
     └───┬───┘      └───┬───┘      └───┬───┘      └───┬───┘
         │              │              │              │
     ┌───┴────┐     ┌───┴────┐     ┌───┴────┐     ┌───┴────┐
     │ 光伏 #1│     │ 储能 #1│     │ 负荷 #1│     │充电桩#1 │
     │ PV     │     │ BAT    │     │ LOAD   │     │CHARGER │
     └────────┘     └────────┘     └────────┘     └────────┘
```

### 2.2 数据模型

```go
package microgrid

// ComponentType 微电网设备类型
type ComponentType string

const (
    CompPV      ComponentType = "pv"        // 光伏发电
    CompBattery ComponentType = "battery"   // 储能电池
    CompLoad    ComponentType = "load"      // 负荷
    CompCharger ComponentType = "charger"   // 充电桩
)

// GridMeterConfig 关口表配置
type GridMeterConfig struct {
    RatedCapacityKW float64 `json:"rated_capacity_kw"` // 并网容量 (kW)
    IslandMode      bool    `json:"island_mode"`        // 孤岛模式
}

// DeviceSwitch 设备开关
type DeviceSwitch struct {
    ID           string `json:"id"`            // 如 "qf-1"
    Name         string `json:"name"`          // 如 "QF1"
    Closed       bool   `json:"closed"`        // true=合闸, false=分闸
    Controllable bool   `json:"controllable"`  // 是否可遥控
}

// DeviceParams 各设备类型专属参数
type DeviceParams struct {
    // --- PV ---
    RatedPowerKW    float64 `json:"rated_power_kw,omitempty"`     // 额定功率 (kW)
    Efficiency      float64 `json:"efficiency,omitempty"`         // 逆变效率 0.7~1.0

    // --- Battery ---
    CapacityKWH     float64 `json:"capacity_kwh,omitempty"`       // 额定容量 (kWh)
    InitSOC         float64 `json:"init_soc,omitempty"`           // 初始SOC 0~100
    SOCMin          float64 `json:"soc_min,omitempty"`            // SOC下限 (%) 默认10
    SOCMax          float64 `json:"soc_max,omitempty"`            // SOC上限 (%) 默认90
    MaxChargeKW     float64 `json:"max_charge_kw,omitempty"`      // 最大充电功率 (kW)
    MaxDischargeKW  float64 `json:"max_discharge_kw,omitempty"`   // 最大放电功率 (kW)
    ChargeEff       float64 `json:"charge_eff,omitempty"`         // 充电效率 0~1
    DischargeEff    float64 `json:"discharge_eff,omitempty"`      // 放电效率 0~1

    // --- Load ---
    LoadRatedKW     float64 `json:"load_rated_kw,omitempty"`      // 额定功率 (kW)
    PowerFactor     float64 `json:"power_factor,omitempty"`       // 功率因数 0~1

    // --- Charger ---
    ChargerRatedKW  float64 `json:"charger_rated_kw,omitempty"`   // 额定功率 (kW)
    ChargerEff      float64 `json:"charger_eff,omitempty"`        // 充电效率 0~1
}

// Device 微电网设备
type Device struct {
    ID     string         `json:"id"`      // 唯一标识
    Type   ComponentType  `json:"type"`    // pv / battery / load / charger
    Name   string         `json:"name"`    // 用户自定义名称
    Switch DeviceSwitch   `json:"switch"`  // 设备开关
    Params DeviceParams   `json:"params"`  // 设备参数
}

// Topology 完整微电网拓扑
type Topology struct {
    GridMeter GridMeterConfig `json:"grid_meter"` // 关口表
    BusName   string          `json:"bus_name"`    // 母线名称, 默认"10kV母线"
    BusVoltageKV float64      `json:"bus_voltage_kv"` // 母线电压
    Devices   []Device        `json:"devices"`     // 设备列表
}
```

### 2.3 拓扑约束

- **唯一关口表**：每个微电网有且仅有 1 个关口表（Grid Meter），作为 PCC 并网点
- **唯一母线**：每个微电网有且仅有 1 条母线，所有设备通过开关接入该母线
- **开关即设备开关**：每个设备与母线之间有一个专用开关，开关不是独立元件
- **设备数量可变**：每种设备的数量为 0~N 个（如可添加 1 个储能或 2 个储能）
- **开关影响功率**：开关分闸 → 对应设备出力/负荷置 0；合闸 → 正常参与功率计算
- **关口表无开关**：关口表到母线始终连通
- **V1.0 支持类型**：光伏(PV)、储能(Battery)、负荷(Load)、充电桩(Charger)

### 2.2 拓扑约束

- 一个微电网实例有且仅有一个**电网接口元件** (grid)，表示 PCC 并网点
- 一个微电网实例至少有 1 个母线，推荐结构：1 条主母线 + 可选子母线
- 元件通过开关/刀闸接入母线，`component.bus_id` 关联到 `switch.id`
- 开关可直接操作分/合，通过 UI 点击或 API/MCP 遥控
- 开关分闸 → 设备出力置 0，测点 QDS 无效
- 开关状态映射为 DI 遥信点，可控开关再映射为 DO 遥控点

### 2.4 点表映射 (IEC104 标准)

微电网拓扑自动展开为 IEC104 标准测点。IOA 按**类型分组**：

| 类型 | IOA 范围 | IEC104 类型标识 | 说明 |
|------|-----------|-----------------|------|
| **AI** (遥测) | 1 ~ 999 | M_ME_NC_1 (13) | 模拟量测量值 |
| **DI** (遥信) | 1001 ~ 1999 | M_SP_NA_1 (1) | 状态量 |
| **AO** (遥调) | 3001 ~ 3999 | C_SE_NC_1 (48) | 远程调节(下行) |
| **DO** (遥控) | 4001 ~ 4999 | C_SC_NA_1 (45) | 远程控制(下行) |

#### 关口表 (Grid Meter — 固定，仅1个)

| IOA | 名称 | 类型 | 说明 | 单位 |
|-----|------|------|------|------|
| 1 | GRID_P | AI | 并网有功 | kW |
| 2 | GRID_Q | AI | 并网无功 | kvar |
| 3 | GRID_V | AI | 电压 | kV |
| 4 | GRID_F | AI | 频率 | Hz |
| 1001 | GRID_Connected | DI | 并网状态(0=离,1=并) | - |
| 1002 | GRID_Island | DI | 孤岛状态 | - |

#### 光伏 PV (第k个，k=0,1,2...)

| IOA | 名称 | 类型 | 说明 | 单位 |
|-----|------|------|------|------|
| 11+k | PV_Power | AI | 发电功率 | kW |
| 21+k | PV_DailyEnergy | AI | 日发电量 | kWh |
| 31+k | PV_TotalEnergy | AI | 总发电量 | kWh |
| 41+k | PV_Irradiance | AI | 辐照度 | W/m² |
| 1011+k | PV_Status | DI | 运行状态(0=停,1=运) | - |
| 1021+k | PV_SwitchStatus | DI | 开关状态(0=分,1=合) | - |
| 4011+k | PV_SwitchControl | DO | 遥控分合(0=分,1=合) | - |

#### 储能 Battery (第k个)

| IOA | 名称 | 类型 | 说明 | 单位 |
|-----|------|------|------|------|
| 51+k | BAT_SOC | AI | 荷电状态 | % |
| 61+k | BAT_Power | AI | 充放电功率(+放-充) | kW |
| 71+k | BAT_Current | AI | 电流 | A |
| 81+k | BAT_Temp | AI | 温度 | °C |
| 1031+k | BAT_Status | DI | 运行状态 | - |
| 1041+k | BAT_ChgState | DI | 充放电态 | - |
| 1051+k | BAT_SwitchStatus | DI | 开关状态 | - |
| 3001+k | BAT_PowerSetpoint | AO | 功率设定值 | kW |
| 4021+k | BAT_SwitchControl | DO | 遥控分合 | - |

#### 负荷 Load (第k个)

| IOA | 名称 | 类型 | 说明 | 单位 |
|-----|------|------|------|------|
| 91+k | LOAD_Power | AI | 有功功率 | kW |
| 101+k | LOAD_Reactive | AI | 无功功率 | kvar |
| 1021+k | LOAD_Status | DI | 运行状态 | - |
| 1031+k | LOAD_SwitchStatus | DI | 开关状态 | - |
| 4031+k | LOAD_SwitchControl | DO | 遥控分合 | - |

#### 充电桩 Charger (第k个)

| IOA | 名称 | 类型 | 说明 | 单位 |
|-----|------|------|------|------|
| 111+k | CHG_Power | AI | 充电功率 | kW |
| 121+k | CHG_Energy | AI | 充电量 | kWh |
| 1041+k | CHG_Status | DI | 运行状态 | - |
| 1051+k | CHG_SwitchStatus | DI | 开关状态 | - |
| 4041+k | CHG_SwitchControl | DO | 遥控分合 | - |

> `k` 表示该类型设备在拓扑中的序号(从0开始)
> 由 `internal/microgrid/pointmap.go` 按类型+索引自动计算，保证唯一性

### 2.5 点表导出

微电网拓扑配置完成后，支持导出为标准 `.xlsx` 点表文件：

- 格式与 GridSim Excel 点表完全兼容
- 包含所有自动展开测点的 IOA、名称、类型、初始值
- 导出文件可直接用于 GridSim 普通 IEC104 实例加载
- 导出位置：`{configDir}/exports/microgrid_{instanceId}_points.xlsx`
- API：`GET /api/v1/microgrid/{id}/export-points`

---

## 3. 物理仿真引擎

### 3.1 仿真循环

```
每 Tick (可配: 100ms ~ 5s):

1. 读取外部输入
   - 环境数据（辐照度、风速、气温）— 来自 CSV 回放、策略生成或API
   - AO/DO 遥控值（来自外部SCADA或MCP）

2. 计算各元件电气量
   - PV: 辐照度 × 额定功率 × 效率 → P_pv
   - Wind: 风速 → 功率曲线 → P_wind
   - Load: 额定功率 × 负荷曲线系数 → P_load
   - Battery: AO设定值 → P_bat (充电为负, 放电为正)
   - Diesel: 运行状态 × 额定功率 → P_diesel

3. 功率平衡计算
   - P_imbalance = ΣP_gen - ΣP_load - P_bat
   - P_imbalance > 0 → 余电上网 (Grid 出口功率)
   - P_imbalance < 0 → 电网取电 (Grid 入口功率)
   - 若孤岛 → 检查是否可平衡，不平衡则切负荷

4. 状态更新
   - SOC: SOC(t+1) = SOC(t) ± (P_bat × Δt / Capacity) × 效率
   - 油量: Fuel(t+1) = Fuel(t) - P_diesel × Δt × 油耗率
   - 电量累计: 各发电/用电元件积分累加
   - 频率/电压: 基于不平衡度简化计算

5. 写入测点
   - 将计算结果写入 library.Store
   - 触发变化上送 (Publish)

6. 事件检测
   - 过载告警
   - SOC 越限
   - 孤岛/并网切换
```

### 3.2 功率平衡算法

```go
// PowerBalanceResult 功率平衡计算结果
type PowerBalanceResult struct {
    TotalGenerationKW  float64  // 总发电 (PV+Wind+Diesel)
    TotalLoadKW        float64  // 总负荷
    BatteryPowerKW     float64  // 电池功率 (+放电, -充电)
    GridPowerKW        float64  // 并网功率 (+购电, -售电)
    ImbalanceKW        float64  // 不平衡量
    Frequency          float64  // 系统频率 (Hz)
    VoltagePU          float64  // 母线电压 (p.u.)
    LoadShed           bool     // 是否切负荷
    Island             bool     // 是否孤岛
}

func (e *Engine) calcPowerBalance() *PowerBalanceResult {
    // 1. 汇总发电
    pvPower     := e.getComponentTotal(CompPV, "power")
    windPower   := e.getComponentTotal(CompWind, "power")
    dieselPower := e.getComponentTotal(CompDiesel, "power")
    genTotal    := pvPower + windPower + dieselPower

    // 2. 汇总负荷
    loadTotal := e.getComponentTotal(CompLoad, "power")

    // 3. 电池功率 (来自AO设定值或自动调度)
    batPower := e.getBatterySetpoint()

    // 4. 不平衡量
    imbalance := genTotal - loadTotal - batPower

    // 5. 电网交互
    gridPower := 0.0
    island    := e.topology.IsIsland()

    if !island {
        if imbalance > gridCapacity {
            // 余电超过并网容量 → 限制出力
            gridPower = gridCapacity
        } else {
            gridPower = imbalance  // 余电上网 / 不足购电
        }
    } else {
        // 孤岛模式: 必须平衡
        if math.Abs(imbalance) > 1e-6 {
            // 不平衡 → 调节电池或切负荷
        }
    }

    // 6. 简化频率/电压计算
    freq := 50.0 + (imbalance / genTotal) * 0.5  // 下垂特性简化
    volt := 1.0 - (loadTotal / e.totalCapacity) * 0.05

    return &PowerBalanceResult{...}
}
```

### 3.3 环境数据源

仿真引擎需要外部环境数据来驱动 PV 和 Wind 出力：

| 数据 | 单位 | 来源 |
|--------|------|--------|
| 辐照度 | W/m² | CSV 回放 / 自动变化策略 / API置数 |
| 风速 | m/s | CSV 回放 / 自动变化策略 / API置数 |
| 气温 | °C | CSV 回放 / API置数 |

环境数据也作为 AI 测点暴露，可以联动现有的 CSV 回放策略或随机变化策略。

---

## 4. 控制与调度逻辑

### 4.1 电池充放电控制

支持 3 种控制模式：

| 模式 | 说明 | 适用场景 |
|--------|------|-------------|
| **定功率** | AO 遥控设定充放电功率 | 外部 EMS 调度 |
| **削峰填谷** | 低电价/光伏高峰时充电，高电价/晚高峰时放电 | 经济调度 |
| **孤岛维持** | 根据系统不平衡自动调节，维持频率稳定 | 离网运行 |

### 4.2 负荷管理

- 负荷可配置为 **固定负荷**（额定功率恒定）或 **动态负荷**（跟随负荷曲线）
- 负荷曲线从 CSV 加载，支持日/周/年模式
- 孤岛模式下支持**自动切负荷**（优先级可配）

### 4.3 并/离网切换

- 通过操作 Connection 的 `Closed` 状态切换
- 并网→离网：微电网进入孤岛模式，电池接管调频
- 离网→并网：检测电网电压/频率，满足条件后同期合闸
- 切换事件记录到 DI 测点

### 4.4 策略引擎复用

微电网的自动变化策略**直接复用** `detail.Engine` 的现有机制：

- 每个元件的 AI 测点可以配置自动变化策略（递增、随机、CSV 回放、自定义公式等）
- 新增的微电网专用策略（光伏辐照度曲线、负荷曲线）通过 `model.StrategyParams` 扩展
- 策略 goroutine 由 `detail.Engine` 统一调度，与现有测点策略无区别

### 4.5 微电网特有策略

| 策略代码 | 名称 | 说明 |
|-----------|------|-------------|
| `microgrid_pv_curve` | 光伏出力曲线 | 根据辐照度+温度计算PV出力 |
| `microgrid_load_profile` | 负荷曲线 | 按日/周负荷曲线输出 |
| `microgrid_battery_schedule` | 电池调度 | 削峰填谷/孤岛调频 |
| `microgrid_island_ctrl` | 孤岛控制 | 检测并网状态，自动切换控制模式 |

---

## 5. Protocol 网关集成

### 5.1 架构原则

微电网本身不是一种"协议"，而是一个仿真引擎。现有 `protocol.Protocol` 接口（IEC104/Modbus）负责**对外通信**，微电网引擎负责**内部仿真计算**。两者的关系：

```
Manager 启动微电网实例时:

   ┌─────────────────────────────────────┐
   │  微电网实例 (Instance struct)          │
   │                                      │
   │  ┌─────────────────────────────────┐ │
   │  │  MicrogridEngine                │ │
   │  │  - 仿真循环 (Tick)              │ │
   │  │  - 功率平衡计算                 │ │
   │  │  - 元件状态更新                 │ │
   │  └──────────┬──────────────────────┘ │
   │             │ 共享 library.Store      │
   │  ┌──────────▼──────────────────────┐ │
   │  │  Protocol (IEC104 / Modbus)     │ │
   │  │  - 对外通信                     │ │
   │  │  - 变化上送                     │ │
   │  └─────────────────────────────────┘ │
   └─────────────────────────────────────┘
```

**核心设计**：
- 微电网实例与普通实例**统一管理**，在 `Manager.instances` 中共存
- **先端口后拓扑**：创建实例时先指定实例名和 IEC104 端口（必填）；拓扑可在创建后配置
- 每个微电网实例对应**唯一 IEC104 端口**（可修改），同一个端口不可重复
- V1.0 仅支持 IEC104 协议
- `Instance` 结构体扩展支持微电网引擎指针

### 5.2 实例创建流程

**核心原则：先创建 → 再配置拓扑 → 保存后启动。创建后实例处于"未配置"状态，无法直接启动。**

```
┌──────────────────────────────────────────────────────────────┐
│  用户在 Web UI / API / MCP 创建微电网实例                     │
│                                                               │
│  Step 1: 填写基本信息 (创建页)                                 │
│   - 实例名称                                                   │
│   - IEC104 端口号（必填，唯一）                                  │
│                                                               │
│  Step 2: 创建实例 (status = "stopped")                        │
│   → POST /api/v1/instances                                    │
│     { protocol: "microgrid", name, iec104_port }              │
│   → 返回 instance_id                                          │
│   → 实例状态: "stopped" (未配置，无法启动)                      │
│                                                               │
│  Step 3: 进入详情页 → 编辑拓扑                                 │
│   - 默认: 空拓扑 (只有母线 + 关口表)                            │
│   - 操作: 添加设备 → 填写参数 → 配置开关                       │
│   - 支持: 添加/删除设备、修改参数、切换开关                     │
│   - 点击"保存配置" → topology 持久化到实例配置                   │
│                                                               │
│  Step 4: 启动实例 (需满足启动条件)                              │
│   - 启动条件: 至少已配置 1 个设备且参数完整                      │
│   - → POST /api/v1/instances/{id}/start                       │
│     自动展开点表 → 启动 IEC104 Server → 启动仿真引擎             │
│   - 状态变为 "running"                                        │
│                                                               │
│  Step 5: 运行中操作                                           │
│   - 运行中不可编辑拓扑 (需停止后修改)                            │
│   - 可操作开关分合 (通过 UI/API/MCP，实时影响功率计算)           │
│   - 停止后可重新编辑拓扑、增减设备、修改参数                     │
└──────────────────────────────────────────────────────────────┘
```

**启动条件检查**：
- ❌ 无任何设备 → 不可启动 (提示"请先添加至少一个设备")
- ❌ 设备参数不完整（额定功率为 0）→ 不可启动 (提示"请完善设备参数")
- ✅ 有关口表 + 至少 1 个设备 + 参数完整 → 可启动

### 5.3 InstanceConfig 扩展

```go
// internal/model/instance.go — 新增字段
type InstanceConfig struct {
    // ... 原有字段 ...
    MicrogridConfig *MicrogridInstanceConfig `json:"microgrid_config,omitempty"`
}

type MicrogridInstanceConfig struct {
    TopologyJSON    string `json:"topology_json,omitempty"`     // 拓扑配置 JSON
    TickMs          int    `json:"tick_ms,omitempty"`            // 仿真步长(ms), 默认1000
    SpeedMultiplier float64 `json:"speed_multiplier,omitempty"`  // 仿真加速比, 默认1.0
    EnvDataFile     string `json:"env_data_file,omitempty"`      // 环境数据CSV
    BaseIOA         uint32 `json:"base_ioa,omitempty"`           // IOA基址(保留,默认0表示自动)
}
```

### 5.4 启动流程

```
Manager.StartMicrogrid(id, cfg):

  1. Load config → 获取 MicrogridConfig + IEC104Port
  2. Check port conflict → 确保端口可用
  3. ParseTopology → 反序列化 TopologyJSON
  4. ExpandPoints → 拓扑 → 测点列表(按 IEC104 标准 IOA 分配)
  5. NewStore(points) → 创建测点存储
  6. NewMicrogridEngine(topology, store, config) → 创建仿真引擎
  7. NewIEC104Wrapper(port) + SetStore(store) → 创建 IEC104 Server
  8. Start IEC104 Server → 开始监听
  9. Start microgrid engine → 开始仿真循环
 10. Start auto-change engine → 加载已有策略
```

### 5.5 兼容性保障

- 仅 `protocol == "microgrid"` 的实例启用微电网模块
- 现有 IEC104/Modbus 普通实例完全不受任何影响
- 同一 Manager 中微电网实例和普通实例共用端口冲突检查

---

## 6. Web UI 设计

### 6.1 实例配置页 (ConfigPage.vue 扩展)

微电网实例在配置列表中以标签 `🔋 微电网` 区分：

- 新建微电网实例：仅需填写**实例名称** + **IEC104 端口号**
- 创建后实例显示状态为"已停止(未配置)"
- 点击进入详情页进行完整拓扑配置
- 实例卡片显示设备数量和配置状态

### 6.2 详情页 — 拓扑编辑 (核心界面)

微电网实例详情页默认进入拓扑编辑视图：

| 区域 | 内容 |
|-----|---------|
| **SVG 拓扑画布** | 单线图：关口表→母线→设备(通过开关连接) |
| **设备面板** | 右侧浮层，列出已添加设备，支持"添加设备"操作 |
| **设备参数弹窗** | 点击设备打开参数配置：名称、额定功率、容量、SOC限值等 |
| **开关交互** | 点击 SVG 开关图标可切换分/合，断开时设备连线变红 |
| **启动按钮** | 满足条件(有设备且参数完整)时高亮可用 |

**拓扑编辑交互流程**：

```
进入详情页 (实例 = 已停止)
  │
  ├─ 默认显示: 空拓扑 (母线 + 关口表 + "开始添加设备"提示)
  │
  ├─ 点击"添加设备" → 选择类型 (PV/储能/负荷/充电桩)
  │    ├─ 新设备出现在 SVG 中，自动分配开关名称 (QF1, QF2...)
  │    └─ 自动打开参数配置弹窗
  │
  ├─ 点击已有设备 → 打开参数配置弹窗
  │    ├─ 设备名称 (用户自定义)
  │    ├─ 设备参数 (按类型显示不同表单)
  │    ├─ [删除设备] 按钮 (红色)
  │    └─ [保存] 按钮
  │
  ├─ 点击设备 SVG 上的开关图标
  │    └─ 开关切换分/合 (仅运行中实时生效，停止时仅作为默认状态)
  │
  ├─ [保存拓扑] 按钮
  │    └─ 持久化到实例配置
  │
  └─ [启动] 按钮 (条件满足时亮起)
       └─ 展开点表 → 启动 IEC104 → 启动仿真
```

### 6.3 详情页 — 其他 Tab

| Tab | 内容 |
|-----|---------|
| **拓扑(主)** | 上述拓扑编辑器 (停止状态可编辑，运行状态只读+开关可操作) |
| **仪表盘** | 运行中显示实时数据卡片 + ECharts 曲线；停止时显示"实例未运行" |
| **点表** | 自动展开的 IEC104 点表，支持导出 .xlsx |
| **设置** | 修改端口、仿真步长、加速比、环境数据等 |

### 6.4 设备参数表单 (按类型)

**PV (光伏)**:
| 字段 | 类型 | 默认 | 说明 |
|-----|------|------|------|
| 名称 | string | PV#k | 设备名称 |
| 额定功率 | float64 | 100 | 额定发电功率 (kW) |
| 逆变效率 | float64 | 0.95 | 逆变效率 0.7~1.0 |

**Battery (储能)**:
| 字段 | 类型 | 默认 | 说明 |
|-----|------|------|------|
| 名称 | string | BAT#k | 设备名称 |
| 额定容量 | float64 | 200 | 额定容量 (kWh) |
| 额定功率 | float64 | 100 | 最大充放电功率 (kW) |
| 初始 SOC | float64 | 50 | 初始荷电状态 0~100% |
| SOC 下限 | float64 | 10 | 最低允许 SOC (%) |
| SOC 上限 | float64 | 90 | 最高允许 SOC (%) |
| 充放电效率 | float64 | 0.95 | 充放电转换效率 |

**Load (负荷)**:
| 字段 | 类型 | 默认 | 说明 |
|-----|------|------|------|
| 名称 | string | LOAD#k | 设备名称 |
| 额定功率 | float64 | 50 | 额定负荷功率 (kW) |
| 功率因数 | float64 | 0.95 | 功率因数 0~1 |

**Charger (充电桩)**:
| 字段 | 类型 | 默认 | 说明 |
|-----|------|------|------|
| 名称 | string | CHG#k | 设备名称 |
| 额定功率 | float64 | 60 | 额定充电功率 (kW) |
| 充电效率 | float64 | 0.90 | 充电效率 0~1 |

### 6.5 拓扑可视化组件

`MicrogridTopology.vue` 组件：

- SVG 渲染单线图：关口表→母线→开关→设备
- 各设备带颜色标识 (PV=绿, BAT=蓝, Load=橙, Charger=紫)
- 开关分闸时设备连线变红/虚线，设备出力值置 0
- 运行中显示实时功率数值悬浮
- 潮流方向动画 (流动圆点)

### 6.6 路由

```
GET  /microgrid/topology/{id}                  → 获取微电网拓扑JSON
PUT  /microgrid/topology/{id}                  → 更新微电网拓扑
POST /microgrid/topology/{id}/device           → 添加设备
DELETE /microgrid/topology/{id}/device/{devId} → 删除设备
PUT  /microgrid/topology/{id}/device/{devId}   → 更新设备参数
POST /microgrid/control/{id}/switch/{devId}    → 遥控开关分合
GET  /microgrid/dashboard/{id}                 → 仪表盘实时数据聚合
```

---

## 7. HTTP API 设计

### 7.1 微电网专用 API

新增路由组 `/api/v1/microgrid/`：

| 方法 | 端点 | 说明 |
|--------|------|-------------|
| `GET` | `/api/v1/microgrid/{id}/topology` | 获取微电网拓扑 |
| `PUT` | `/api/v1/microgrid/{id}/topology` | 更新微电网拓扑（JSON） |
| `POST` | `/api/v1/microgrid/{id}/reset` | 重置仿真状态 |
| `POST` | `/api/v1/microgrid/{id}/switch` | 并/离网切换 `{"island": true}` |
| `POST` | `/api/v1/microgrid/{id}/load-shed` | 手动切负荷 `{"load_id":"...","shed":true}` |
| `GET` | `/api/v1/microgrid/{id}/dashboard` | 仪表盘聚合数据 |
| `POST` | `/api/v1/microgrid/{id}/env-data` | 上传环境数据CSV |
| `GET` | `/api/v1/microgrid/{id}/history` | 获取仿真历史数据 (时序) |

### 7.2 MCP 工具扩展

| 工具名 | 说明 |
|-----------|-------------|
| `create_microgrid` | 创建微电网实例 |
| `config_microgrid_topology` | 配置微电网拓扑 |
| `microgrid_switch` | 并/离网切换 |
| `microgrid_load_shed` | 负荷投切 |
| `get_microgrid_dashboard` | 获取微电网仪表盘 |

---

## 8. 数据存储与持久化

### 8.1 拓扑持久化

微电网拓扑以 JSON 格式存储在实例配置中（`instances.json` 的 `MicrogridConfig.TopologyJSON` 字段），与现有实例配置持久化机制一致。

### 8.2 仿真历史记录

仿真历史采用环形内存缓冲区，保留最近 N 帧快照（默认 3600 帧，即 1h @ 1s Tick）：

```go
type HistoryFrame struct {
    Timestamp time.Time
    Snapshot  map[string]float64  // component_id → 关键电气量
}

type HistoryBuffer struct {
    mu       sync.Mutex
    maxSize  int
    frames   []HistoryFrame
    cursor   int
    wrapped  bool
}
```

通过 `GET /api/v1/microgrid/{id}/history` 查询，支持 ECharts 前端展示历史曲线。

### 8.3 CSV 环境数据

环境数据 CSV 格式示例：

```csv
time,irradiance_wm2,wind_speed_ms,temperature_c
00:00:00,0,2.1,15
00:05:00,50,2.3,15.5
...
```

- 放置于实例配置目录 `{configDir}/{instanceID}/env/` 
- 可通过 `POST /api/v1/microgrid/{id}/env-data` 上传
- 与环境数据测点联动，复用 CSV 回放策略机制

---

## 9. 实现计划

### 阶段 1 — 核心引擎 (预计 3-4 天)

| # | 任务 | 文件 |
|---|------|------|
| 1.1 | 定义微电网数据模型 (Device/DeviceSwitch/Topology) | `internal/microgrid/model.go` |
| 1.2 | 实现仿真引擎核心循环 | `internal/microgrid/engine.go` |
| 1.3 | 实现功率平衡计算 (开关感知) | `internal/microgrid/powerflow.go` |
| 1.4 | 拓扑 → 点表自动展开 | `internal/microgrid/pointmap.go` |
| 1.5 | 仿真历史缓冲区 | `internal/microgrid/history.go` |

### 阶段 2 — 集成与 API (预计 2-3 天)

| # | 任务 | 文件 |
|---|------|------|
| 2.1 | 扩展 InstanceConfig + factory | `internal/model/instance.go`, `pkg/protocol/factory.go` |
| 2.2 | 微电网 HTTP API 路由 (CRUD + 控制) | `cmd/gridsim/main.go` 扩展 |
| 2.3 | MCP 工具扩展 | `internal/mcp/server.go` |
| 2.4 | 拓扑持久化 + 配置加载 | `internal/microgrid/store.go` |

### 阶段 3 — Web UI (预计 3-4 天)

| # | 任务 | 文件 |
|---|------|------|
| 3.1 | 配置页微电网创建 | `web/src/views/ConfigPage.vue` |
| 3.2 | 拓扑编辑器 (添加/删除设备, SVG 渲染) | `web/src/views/MicrogridEditor.vue` |
| 3.3 | 设备参数配置弹窗 | `web/src/views/MicrogridEditor.vue` |
| 3.4 | 微电网详情页 + Tab 整合 | `web/src/views/MicrogridDetail.vue` |
| 3.5 | 仿真仪表盘 ECharts | `web/src/views/MicrogridDashboard.vue` |
| 3.6 | 前端 API 扩展 | `web/src/api/index.ts` |

### 阶段 4 — 测试与文档 (预计 2 天)

| # | 任务 |
|---|------|
| 4.1 | 单元测试：引擎 + 功率平衡 |
| 4.2 | 集成测试：微电网实例生命周期 |
| 4.3 | 端到端：IEC104 暴露微电网测点 |
| 4.4 | 文档更新 |

---

## 10. 约束与注意事项

### 10.1 技术约束

- **仿真精度**：电气计算为简化模型（稳态），非电磁暂态，目标为功能测试和策略验证
- **并发限制**：`detail.Engine` 限制了 `maxConcurrentTasks = 100`，微电网策略纳入同一计数
- **加速比限制**：`SpeedMultiplier <= 100`，防止 CPU 过载

### 10.2 向后兼容

- 现有 `protocol != "microgrid"` 的实例完全不受影响
- 不修改 `protocol.Protocol` 接口定义，仅新增实现
- 不修改 `InstanceConfig` 的已有字段（仅新增 `MicrogridConfig *` 指针字段，nil 表示非微电网）

### 10.3 扩容性分析

| 场景 | 估算上限 | 制约因素 |
|---------|-----------|----------------|
| **微电网实例数** | 50 / 台物理机 | 每个实例占用 1 个 IEC104 端口 + 1 个仿真 goroutine + 独立 Store |
| **每个微电网的元件数** | 50 个 | 按每元件 10 个测点计算，共 500 测点，单 Store 可轻松承载 |
| **同时运行数** | 50 (全部) | 假设 Tick=1s，每个 Tick 循环计算量 <1ms，50 个实例约 50ms 计算 |
| **Manager 总实例上限** | 不单独限制 | 沿用全局 `MaxInstances = 1000`，其中微电网最多 50 |
| **内存占用** | ~5MB / 实例 | Store 500 点 + History 3600 帧 + 拓扑数据 |
| **端口占用** | 50 个 | 每个微电网 1 个 IEC104 TCP 端口 |

> 微电网实例上限可通过 `MicrogridMaxInstances` 配置（默认 50）

### 10.4 已知限制 (v1.0)

- 仅支持 IEC104 协议（后续版本增加 Modbus）
- 不支持三相不平衡
- 无功功率/电压调节为简化模型
- 仅支持单母线单进线拓扑
- 不支持柴油发电机、风力发电（后续版本）
- 不支持并联设备功率分配

---

## 11. 附录：点表 IOA 分配示例

一个包含 关口表 + PV×1 + Battery×1 + Load×1 + Charger×1 的微电网，按 IEC104 标准 IOA 分配：

| IOA | 名称 | 类型 | 元件 | 说明 |
|-----|------|------|---------|--------|
| 1 | GRID_P | AI | 关口表 | 并网有功 (kW) |
| 2 | GRID_Q | AI | 关口表 | 并网无功 (kvar) |
| 3 | GRID_V | AI | 关口表 | 电压 (kV) |
| 4 | GRID_F | AI | 关口表 | 频率 (Hz) |
| 1001 | GRID_Connected | DI | 关口表 | 并网状态 |
| 1002 | GRID_Island | DI | 关口表 | 孤岛状态 |
| 11 | PV1_Power | AI | 光伏#1 | 发电功率 (kW) |
| 21 | PV1_DailyEnergy | AI | 光伏#1 | 日发电量 (kWh) |
| 31 | PV1_TotalEnergy | AI | 光伏#1 | 总发电量 (kWh) |
| 41 | PV1_Irradiance | AI | 光伏#1 | 辐照度 (W/m²) |
| 1011 | PV1_Status | DI | 光伏#1 | 运行状态 |
| 1021 | PV1_SwitchStatus | DI | 光伏#1 | 开关QF1状态 |
| 4011 | PV1_SwitchControl | DO | 光伏#1 | 遥控QF1 |
| 51 | BAT1_SOC | AI | 储能#1 | 荷电状态 (%) |
| 61 | BAT1_Power | AI | 储能#1 | 充放电功率 (kW) |
| 71 | BAT1_Current | AI | 储能#1 | 电流 (A) |
| 81 | BAT1_Temp | AI | 储能#1 | 温度 (°C) |
| 1031 | BAT1_Status | DI | 储能#1 | 运行状态 |
| 1041 | BAT1_ChgState | DI | 储能#1 | 充放电态 |
| 1051 | BAT1_SwitchStatus | DI | 储能#1 | 开关QF2状态 |
| 3001 | BAT1_PowerSetpoint | AO | 储能#1 | 功率设定值 |
| 4021 | BAT1_SwitchControl | DO | 储能#1 | 遥控QF2 |
| 91 | LOAD1_Power | AI | 负荷#1 | 有功功率 (kW) |
| 101 | LOAD1_Reactive | AI | 负荷#1 | 无功功率 (kvar) |
| 1021 | LOAD1_Status | DI | 负荷#1 | 运行状态 |
| 1031 | LOAD1_SwitchStatus | DI | 负荷#1 | 开关QF3状态 |
| 4031 | LOAD1_SwitchControl | DO | 负荷#1 | 遥控QF3 |
| 111 | CHG1_Power | AI | 充电桩#1 | 充电功率 (kW) |
| 121 | CHG1_Energy | AI | 充电桩#1 | 充电量 (kWh) |
| 1041 | CHG1_Status | DI | 充电桩#1 | 运行状态 |
| 1051 | CHG1_SwitchStatus | DI | 充电桩#1 | 开关QF4状态 |
| 4041 | CHG1_SwitchControl | DO | 充电桩#1 | 遥控QF4 |

**总计：30 个测点** (AI: 12, DI: 13, AO: 1, DO: 4)

---

> **文档版本记录**
> | 版本 | 日期 | 变更说明 |
> |-------|--------|-----------|
> | v2.0 | 2026-05-23 | 重构：单母线单关口拓扑、设备+开关合并模型、充电桩支持、创建→配置→启动工作流、新增充电桩参数和点表 |
> | v1.1 | 2026-05-23 | 修正：IEC104 标准 IOA 分组、新增开关元件、端口优先创建、扩容性分析、点表导出 |
> | v1.0 | 2026-05-23 | 初版设计稿 |
