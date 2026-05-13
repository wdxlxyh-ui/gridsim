# v2.1 后端设计方案

## 一、概述

为运行中的实例提供 **详情页** 支持，包含：
- 测点实时值高频刷新（最快 100ms）
- 置数（写入点值并触发 IEC104 变化上送）
- 自动变化（9 种策略后台定时计算并写入：递增、随机、CSV、MAX、MIN、SOC计算、电量计算、**AO关联**、**接口更新**）
- 批量配置自动变化、导出/导入自动变化配置
- 测点数据 CSV 导出

---

## 二、新增模块结构

```
internal/
├── detail/                    ← 新增：详情页 & 自动变化引擎
│   ├── engine.go              ← 自动变化调度引擎 (核心)
│   ├── strategy.go            ← 9 种策略的计算逻辑
│   ├── csv_player.go          ← CSV 回放播放器
│   └── store.go               ← 自动变化配置持久化
├── manager/manager.go         ← 修改：注册 detail handler
└── model/
    ├── instance.go             ← 不变
    └── detail.go               ← 新增：详情页相关数据模型

pkg/api/
└── handler.go                 ← 不变（legacy 点表 API）

cmd/iec104-sim/
└── main.go                    ← 修改：注册 /api/v1/instances/{id}/points 路由
```

---

## 三、数据模型

### 3.1 AutoChangeConfig（自动变化配置）

```go
// 存储于 config/auto_changes/{instance_id}.json
type AutoChangeConfig struct {
    PointIOA   uint32           `json:"ioa"`
    Strategy   string           `json:"strategy"`   // increment|random|csv|max|min|soc|energy|aofollow|apiupdate
    Enabled    bool             `json:"enabled"`
    Params     StrategyParams   `json:"params"`
    UpdatedAt  time.Time        `json:"updated_at"`
}

type StrategyParams struct {
    // 递增
    StartValue   float64 `json:"start_value,omitempty"`
    Step         float64 `json:"step,omitempty"`
    PeriodMs     int     `json:"period_ms,omitempty"`     // >= 100
    MaxValue     float64 `json:"max_value,omitempty"`

    // 随机
    MinValue     float64 `json:"min_value,omitempty"`
    MaxValue     float64 `json:"max_value,omitempty"`
    DecimalPlaces int    `json:"decimal_places,omitempty"` // 0 或 1

    // CSV
    CSVFileName  string  `json:"csv_file,omitempty"`
    TimeFormat   string  `json:"time_format,omitempty"`  // absolute|relative
    TimeUnit     string  `json:"time_unit,omitempty"`    // s|ms

    // MAX / MIN
    ParaA        string  `json:"para_a,omitempty"`       // IOA列表，分号间隔
    ParaB        string  `json:"para_b,omitempty"`       // 关联IOA

    // SOC
    InitSOC      float64 `json:"init_soc,omitempty"`     // %
    RatedCap     float64 `json:"rated_cap,omitempty"`    // kWh
    PowerIOA     uint32  `json:"power_ioa,omitempty"`    // 功率点号
    IntegralMs   int     `json:"integral_ms,omitempty"`  // 积分周期

    // 电量
    InitEnergy   float64 `json:"init_energy,omitempty"`  // kWh
    StatType     int     `json:"stat_type,omitempty"`    // 0=充电 1=放电
    EnergyPowerIOA uint32 `json:"energy_power_ioa,omitempty"`
    EnergyPeriodMs int   `json:"energy_period_ms,omitempty"`

    // AO 关联
    FollowAOIOA  uint32 `json:"follow_ao_ioa,omitempty"`  // 关联的 AO 点号

    // 接口更新
    APIInitValue float64 `json:"api_init_value,omitempty"` // 初始值
}
```

### 3.2 PointSnapshot（实时测点快照，API 返回用）

```go
type PointSnapshot struct {
    IOA       uint32    `json:"ioa"`
    Name      string    `json:"name"`
    PointType string    `json:"point_type"` // AI/DI/PI/DO/AO
    Value     float64   `json:"value"`
    BoolValue bool      `json:"bool_value"`
    IntValue  int32     `json:"int_value"`
    UpdatedAt time.Time `json:"updated_at"`  // 毫秒精度
    Unit      string    `json:"unit"`        // 从配置表衍生
}
```

---

## 四、核心引擎设计

### 4.1 AutoChangeEngine

```
┌─────────────────────────────────────────┐
│           AutoChangeEngine              │
│─────────────────────────────────────────│
│  - mu sync.RWMutex                      │
│  - tasks map[ioa]*ChangeTask            │
│  - store *AutoChangeStore               │
│  - iecPub Publisher (触发变化上送)       │
│  - pointStore *library.Store            │
└──────────┬──────────────────────────────┘
           │
    ┌──────┴──────┐
    │  ChangeTask  │  (goroutine per task)
    │──────────────│
    │  ticker      │
    │  calc()      │ ← 调用 Strategy 计算
    │  write()     │ ← 写入 pointStore + 触发上送
    └──────────────┘
```

**生命周期：**

| 动作 | 行为 |
|------|------|
| 实例启动 | 从 `auto_changes/{id}.json` 加载配置，启动所有 `enabled=true` 的任务 |
| 实例停止 | 取消所有任务的 goroutine，保存当前状态 |
| 前端配置自动变化 | PUT API → 更新配置 → 重启对应任务的 goroutine |
| 前端关闭自动变化 | DELETE API → 停止 goroutine → 删除配置 |

### 4.2 9 种策略计算逻辑

#### a. 递增 (Increment)
```
周期触发:
  current = store.Get(ioa).Value
  next = current + step
  if next > maxValue: next = startValue
  store.SetValue(ioa, next)
  publisher.Publish(point)  // 触发 IEC104 变化上送
```

#### b. 随机 (Random)
```
周期触发:
  value = min + rand.Float64() * (max - min)
  if decimalPlaces == 1: value = round(value, 1)
  else: value = round(value, 0)
  store.SetValue(ioa, value)
  publisher.Publish(point)
```

#### c. CSV 回放
```
启动时:
  csvPath = configDir + "/csv/" + csvFileName
  rows = parseCSV(csvPath)  // [{time, value}]
  if timeFormat == "absolute":
    按 hh:mm:ss 定位当前时刻对应的行，顺序播放到末尾停止
  if timeFormat == "relative":
    index = 0
    周期触发:
      row = rows[index]
      store.SetValue(ioa, row.value)
      publisher.Publish(point)
      time.Sleep(row.time)  // 使用 CSV 中指定的间隔
      index = (index + 1) % len(rows)  // 循环
```

#### d. MAX
```
周期触发:
  ioas = parseParaA(paraA)  // 分号分割
  values = []
  for _, ioa := range ioas:
      values = append(values, store.Get(ioa).Value)
  result = max(values)

  if paraB != "":
     关联点 = store.Get(paraB)
     if 关联点.BoolValue == false:   // 关联点为 0
         result = 0

  store.SetValue(targetIOA, result)
  publisher.Publish(point)
```

#### e. MIN
```
同 MAX，取 min(values) 而非 max(values)
```

#### f. SOC 计算
```
初始:
  currentSOC = initSOC

周期触发:
  power = store.Get(powerIOA).Value  // kW
  T = integralMs / 1000.0            // 转为秒
  deltaSOC = power * T / ratedCap * 100
  currentSOC = currentSOC + deltaSOC
  if currentSOC > 100: currentSOC = 100
  if currentSOC < 0:   currentSOC = 0
  store.SetValue(targetIOA, currentSOC)
  publisher.Publish(point)
```

#### g. 电量计算
```
初始:
  currentEnergy = initEnergy

周期触发:
  power = store.Get(powerIOA).Value
  T = integralMs / 3600000.0         // 转为小时
  if statType == 0 (充电) AND power > 0:
      currentEnergy += power * T
  if statType == 1 (放电) AND power < 0:
      currentEnergy += abs(power) * T
  store.SetValue(targetIOA, currentEnergy)
  publisher.Publish(point)
```

#### h. AO 关联 (AO Follow)
```
触发条件:
  实例运行时，每当指定 AO 点被遥控（C_SE_NC_1 控制帧到达）

行为:
  aoValue = store.Get(followAOIOA).Value
  store.SetValue(targetIOA, aoValue)
  publisher.Publish(point)

说明:
  - AO 关联不是定时轮询，而是被动触发
  - 在 IEC104 Server 的 HandleControl() 中检测目标 AO 点被控制后，
    同步更新关联的 AI/DI/PI 点
  - 无独立 ticker goroutine，注册回调到 AO 点的 control handler
```

#### i. 接口更新 (API Update)
```
行为:
  - 此模式仅接受通过 HTTP API（PUT /api/points/{ioa}）写入值
  - 自动变化引擎不做任何周期性计算
  - 任何非 API 方式的写入（包含其他自动变化策略写入）将被拒绝
  - 启动时以初始值（api_init_value）作为当前值

API 写入流程:
  1. 收到 PUT /api/v1/instances/{id}/points/{ioa} 请求
  2. 检测该点策略是否为 apiupdate
  3. 若是，允许写入并触发 IEC104 变化上送
  4. 若非，返回错误："该点配置了接口更新策略，仅能通过接口写入"

注意:
  - 此策略用于控制哪些点允许被外部系统通过 API 修改
  - 配置了其他策略的点，接口写入会被拒绝
```

---

## 五、新增 API 端点

### 5.1 实例详情页高频查询

```
GET /api/v1/instances/{id}/points
```

返回该实例所有测点的实时快照（毫秒级时间戳）：

```json
{
  "points": [
    {
      "ioa": 16385,
      "name": "母线电压",
      "point_type": "AI",
      "value": 100.50,
      "bool_value": false,
      "int_value": 0,
      "updated_at": "2026-05-12T10:00:01.234Z",
      "unit": "kV"
    }
  ],
  "refreshed_at": "2026-05-12T10:00:01.234Z"
}
```

### 5.2 置数

```
PUT /api/v1/instances/{id}/points/{ioa}
Content-Type: application/json

{ "value": 235.5 }         // AI/AO
{ "bool_value": true }     // DI/DO
{ "int_value": 42 }        // PI
```

行为：写入 pointStore → 触发 IEC104 变化上送（COT=3）

### 5.3 获取自动变化配置

```
GET /api/v1/instances/{id}/points/{ioa}/auto-change
```

### 5.4 配置自动变化

```
PUT /api/v1/instances/{id}/points/{ioa}/auto-change
Content-Type: application/json

{
  "strategy": "increment",
  "enabled": true,
  "params": {
    "start_value": 0,
    "step": 1,
    "period_ms": 1000,
    "max_value": 100
  }
}
```

行为：保存配置 → 停止旧任务（如有）→ 启动新任务（如 enabled=true）

### 5.5 删除自动变化配置

```
DELETE /api/v1/instances/{id}/points/{ioa}/auto-change
```

行为：停止任务 → 删除配置

### 5.6 上传 CSV

```
POST /api/v1/instances/{id}/upload-csv
Content-Type: multipart/form-data

file: <upload.csv>
```

保存至 `config/csv/{instance_id}/{filename}`

### 5.7 导出测点数据

```
GET /api/v1/instances/{id}/points/export
Accept: text/csv
```

返回 CSV：
```
信息体地址,测点名称,测点类型,实时值,测点值更新时间
16385,母线电压,AI,100.50,2026-05-12 10:00:01.234
```

### 5.8 批量配置自动变化

```
PUT /api/v1/instances/{id}/points/auto-change/batch
Content-Type: application/json

{
  "ioas": [16385, 16386, 16387],
  "config": {
    "strategy": "increment",
    "enabled": true,
    "params": {
      "start_value": 0,
      "step": 1,
      "period_ms": 1000,
      "max_value": 100
    }
  }
}
```

行为：对指定 IOA 列表逐一应用相同的自动变化配置，原子性操作（失败全部回滚）。

### 5.9 导出自动变化配置

```
GET /api/v1/instances/{id}/points/auto-change/export
Accept: application/json
```

返回 JSON：
```json
{
  "autoChanges": {
    "16385": { "strategy": "increment", "enabled": true, "params": { ... } },
    "16386": { "strategy": "random", "enabled": true, "params": { ... } }
  },
  "exportTime": "2026-05-13T10:00:00.000Z",
  "instanceId": "instance-1"
}
```

### 5.10 导入自动变化配置

```
POST /api/v1/instances/{id}/points/auto-change/import
Content-Type: application/json

{
  "autoChanges": {
    "16385": { "strategy": "increment", "enabled": true, "params": { ... } },
    "16386": { "strategy": "random", "enabled": true, "params": { ... } }
  }
}
```

行为：解析导入的 JSON，逐个配置自动变化。仅对配置中存在的 IOA 生效，不影响未包含的 IOA。

---

## 六、实例生命周期管理变更

修改 `internal/manager/manager.go`：

```go
type Instance struct {
    Config     model.InstanceConfig
    Server     *iec104.Server
    Store      *library.Store
    HTTPServer *http.Server
    AutoEngine *detail.Engine       // ← 新增
}

func (m *Manager) StartInstance(id string) error {
    // ... 现有逻辑 ...
    engine := detail.NewEngine(store, server, m.cfgDir)
    engine.LoadAndStart(id)          // 从磁盘加载自动变化配置并启动
    inst.AutoEngine = engine
}

func (m *Manager) StopInstance(id string) error {
    // ... 现有逻辑 ...
    if inst.AutoEngine != nil {
        inst.AutoEngine.StopAll()    // 停止所有自动变化任务
    }
}

func (m *Manager) RegisterDetailRoutes(mux *http.ServeMux, id string) {
    // 注册 /api/v1/instances/{id}/points/* 路由
    // 委托给 detail.Handler
}
```

---

## 七、并发安全

| 资源 | 保护方式 |
|------|---------|
| pointStore 写操作 | 已使用 `sync.RWMutex` |
| auto-change 配置读写 | `engine.mu sync.RWMutex` |
| 每个任务的 goroutine | 通过 `context.Context` + `cancel()` 控制生命周期 |
| CSV 文件读取 | 启动时一次性加载到内存，后续无文件 IO |

---

## 八、与前端交互流程

```
前端                               后端
  │                                  │
  │  GET /points (200ms轮询)          │
  │ ──────────────────────────────►  │
  │  ◄───── PointSnapshot[] ──────  │
  │                                  │
  │  置数: PUT /points/{ioa}         │
  │ ──────────────────────────────►  │
  │  ◄───── { success: true } ────  │ —→ 触发 IEC104 变化上送
  │                                  │
  │  配置自动变化: PUT .../auto-change│
  │ ──────────────────────────────►  │
  │  ◄───── { ok } ───────────────  │ —→ 启动后台 goroutine
  │                                  │     │
  │                                  │  ←─ 周期计算 → 写 pointStore
  │                                  │     │
  │  下次轮询获取到新值                │  ←─ Publish → IEC104 上送
  │                                  │
```

---

## 九、关键约束

| 项目 | 约束 |
|------|------|
| 变化周期最小值 | **100ms**（所有 period_ms 字段校验） |
| 小数位数 | 仅支持 **整数** 或 **1位小数** |
| CSV 文件 | 2 列：Time, value；编码 UTF-8 |
| CSV 绝对时刻 | 格式 `hh:mm:ss` |
| CSV 相对时刻 | Time 列为整数（ms），持续循环播放 |
| 最大并发任务数 | 不超过实例测点总数 |
| AO/DO | 不支持置数和自动变化（置数列和自动变化列显示 `—`） |
| DI 置数 | 仅能置 0 或 1，界面以 ON/OFF 开关展示 |
| 置数显示 | 置数值存储于独立字段，不随实时数据刷新，仅置数操作时更新 |
| 置数确认 | 置数无需二次确认，成功后以绿色 toast 提示 |
| 接口更新策略 | 配置了此策略的点只能通过 HTTP API 写入，其他方式拒绝并返回错误 |
| AO 关联策略 | 无独立 ticker，注册为 AO 点控制回调，被动触发 |
