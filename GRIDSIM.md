# GridSim — 多协议电网仿真平台

GridSim 是一个 IEC104/Modbus TCP 双协议从站模拟器，支持多实例并行运行、微电网仿真、11种自动变化策略，并提供 REST API 和 MCP Server 两种控制接口。

---

## 1. 项目结构

```
gridsim
├── cmd/gridsim/         入口: 传统模式(-p -c) / 服务模式(serve)
├── internal/
│   ├── detail/          详情页: 自动变化策略引擎 + 测点读写API
│   ├── manager/         多实例生命周期管理 (最多1000个)
│   ├── mcp/             MCP Server (list_instances, write_points 等28个工具)
│   ├── microgrid/       微电网仿真引擎 (PV/电池/负荷/充电桩 + 关口表 + 公式)
│   ├── model/           数据模型 (InstanceConfig, AutoChangeConfig, PointSnapshot)
│   └── storage/         JSON 持久化存储
├── pkg/
│   ├── api/             传统模式 HTTP API (/api/points, /api/status)
│   ├── config/          点表模型 (Point) + XLSX加载器
│   ├── firewall/        iptables 端口管理
│   ├── iec104/          IEC104 服务端 (总召/电度召唤/遥控/遥调/变化上送)
│   ├── library/         并发安全内存点表 (Store, CollectChanged)
│   ├── middleware/      HTTP 中间件 (JWT认证, Panic恢复)
│   └── protocol/        协议接口 + IEC104包装 + Modbus TCP
├── web/src/views/       Vue 3 前端页面
│   ├── ConfigPage.vue       实例管理 (CRUD + 启停)
│   ├── DetailPage.vue       测点详情 (轮询/置数/11种策略/CSV回放)
│   ├── TrendPage.vue        ECharts趋势对比
│   ├── MonitorPage.vue      运行监控
│   └── MicrogridEditor.vue  微电网编辑 (拓扑/设备/关口表/公式)
└── config/              运行配置目录 (instances.json, users.json)
```

---

## 2. 数据模型

### Point (测点)

```go
type Point struct {
    IOA       uint32    // 信息体地址 (唯一标识)
    Name      string    // 测点名称 (如 "关口表_有功功率")
    ValueType ValueType // FLOAT / DOUBLE / INT / BIT
    PointType PointType // AI(遥测) / DI(遥信) / PI(遥脉) / DO(遥控) / AO(遥调)
    Value     float64   // 浮点数值
    BoolValue bool      // 布尔值 (DI/DO)
    IntValue  int32     // 整数值 (PI)
    Efficient float64   // 系数
    BaseValue float64   // 初始值
    QDS       struct {  // 品质描述
        Invalid, NotTopical, Substituted, Overflow, Blocked bool
    }
    Timestamp time.Time // 最后更新时间
    Changed   bool      // 是否有变化 (供 CollectChanged 使用)

    // Modbus 扩展
    FunctionCode    uint8  // 功能码 (1/3/4/5/6/16)
    RegisterAddress uint16 // 寄存器地址
    ByteOrder       string // 字节序
}
```

### PointSnapshot (测点快照, API返回)

```go
type PointSnapshot struct {
    IOA       uint32    // 信息体地址
    Name      string    // 名称
    PointType string    // AI/DI/PI/DO/AO
    Value     float64   // 值
    BoolValue bool
    IntValue  int32
    UpdatedAt time.Time // 更新时间戳
    Unit      string    // 单位 (从 Alias 解析)

    FunctionCode    uint8  // Modbus 功能码
    RegisterAddress uint16 // Modbus 寄存器地址
    ByteOrder       string
}
```

### InstanceConfig (实例配置)

```go
type InstanceConfig struct {
    ID              string // 自动生成的唯一 ID
    Name            string // 实例名称
    IEC104Port      int    // IEC104 TCP 端口
    XLSXFile        string // 点表文件 (.xlsx)
    Enabled         bool
    HttpEnabled     bool   // 是否启用独立 HTTP API
    HttpPort        int
    Protocol        string // "iec104" / "modbus_tcp" / "microgrid"
    ModbusConfig    *ModbusInstanceConfig    // Modbus 参数
    MicrogridConfig *MicrogridInstanceConfig // 微电网参数
}

type ModbusInstanceConfig struct {
    Port      int    // Modbus TCP 端口
    ByteOrder string // 字节序 (ABCD/BADC/CDAB/DCBA)
    SlaveID   uint8  // 从站 ID
}

type MicrogridInstanceConfig struct {
    TopologyJSON string  // 微电网拓扑 JSON
    PointsJSON   string  // 测点映射 JSON
    TickMs       int     // 仿真 tick 周期 (毫秒)
    SpeedFactor  float64 // 仿真速度倍率
}
```

### InstanceState (运行状态, API返回)

```go
type InstanceState struct {
    Config           InstanceConfig
    Status           string // "stopped" / "running" / "error"
    UptimeSeconds    int64
    TotalPoints      int
    ClientConnected  bool   // IEC104 客户端是否在线
    Interrogations   int64  // 总召唤次数
    Controls         int64  // 遥控/遥调次数
    Spontaneous      int64  // 变化上送次数
    Error            string
}
```

---

## 3. 协议支持

### 3.1 协议接口

所有协议实现统一的 `Protocol` 接口，可在同一端口并行工作：

```go
type Protocol interface {
    Name() string                              // "iec104" / "modbus_tcp"
    Start() error                              // 启动协议服务端
    Stop()                                     // 停止
    ClientConnected() bool                     // 是否有客户端连接
    ClientAddr() string                        // 客户端地址
    Stats() (interrog, control, spont int64)   // 运行统计
    Uptime() int64                             // 运行时长(秒)
    Publish(point *config.Point)               // 触发变化上送
    SetStore(store *library.Store)             // 设置点表
}
```

支持的协议（由 `GET /api/v1/protocols` 返回）:
- `iec104` — IEC 60870-5-104 从站
- `modbus_tcp` — Modbus TCP 从站
- `microgrid` — 基于 IEC104 的微电网仿真模式

### 3.2 IEC104 协议

- **ASDU 类型支持**: M_ME_NC_1(13遥测), M_SP_NA_1(1遥信), M_IT_NA_1(15遥脉), C_SC_NA_1(45遥控), C_SE_NC_1(48遥调)
- **总召唤**: C_IC_NA_1(100) — 按 AI→DI→AO→DO 顺序回传所有点
- **电度召唤**: C_CI_NA_1(101)
- **变化上送**: COT=3, 通过 `Publish()` 触发, 经 `publishCh` 通道异步发送
- **品质描述 QDS**: 支持 invalid / not_topical / substituted / overflow / blocked
- **单客户端限制**: 每个实例只接受一个 IEC104 客户端连接

### 3.3 Modbus TCP 协议

- **功能码**: 1(读线圈), 3(读保持寄存器), 4(读输入寄存器), 5(写单线圈), 6(写单寄存器), 16(写多寄存器)
- **字节序**: ABCD / BADC / CDAB / DCBA 可配置
- **数据映射**: 寄存器地址 ↔ IOA 通过点表配置
- *注: Modbus TCP 的 `Publish()` 仅递增计数器，不主动推送到客户端(Modbus 无服务器推送机制)*

---

## 4. REST API

### 4.1 管理 API (服务模式 `/api/v1/`)

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/auth/login` | 用户登录, 返回 JWT token |
| `GET` | `/instances` | 列出所有实例状态 |
| `POST` | `/instances` | 创建新实例配置 |
| `GET` | `/instances/{id}` | 获取实例详情 |
| `PUT` | `/instances/{id}` | 更新实例配置 |
| `DELETE` | `/instances/{id}` | 删除实例配置 |
| `POST` | `/instances/{id}/start` | 启动实例 |
| `POST` | `/instances/{id}/stop` | 停止实例 |
| `POST` | `/instances/{id}/restart` | 重启实例 |
| `GET` | `/status` | 全局服务状态 (版本/实例数/运行数) |
| `POST` | `/upload` | 上传 .xlsx / .csv 点表文件 |
| `GET` | `/files` | 列出 config 目录下的 .xlsx 文件 |
| `GET` | `/protocols` | 查询支持的协议列表 |

### 4.2 实例测点 API (服务模式)

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/instances/{id}/points` | 列出所有测点快照 (AI优先+IOA升序) |
| `GET` | `/instances/{id}/points/{ioa}` | 获取单点快照 |
| `PUT` | `/instances/{id}/points/{ioa}` | 置数 (value/bool_value/int_value) |
| `GET` | `/instances/{id}/points/batch?ioas=1,2,3` | **批量读取** 指定IOA快照 |
| `POST` | `/instances/{id}/points/batch` | **批量写入** 多个测点值 |
| `GET` | `/instances/{id}/points/export` | 导出测点 CSV |
| `GET` | `/instances/{id}/points/auto-change/{ioa}` | 查看自动变化配置 |
| `PUT` | `/instances/{id}/points/auto-change/{ioa}` | 配置自动变化 |
| `DELETE` | `/instances/{id}/points/auto-change/{ioa}` | 删除自动变化配置 |
| `PUT` | `/instances/{id}/points/auto-change/batch` | 批量配置自动变化 |
| `GET` | `/instances/{id}/points/auto-change/export` | 导出所有自动变化配置 (CSV) |
| `POST` | `/instances/{id}/points/auto-change/import` | 导入自动变化配置 |
| `POST` | `/instances/{id}/upload-csv` | 上传 CSV 回放文件 |
| `GET` | `/instances/{id}/csv-files` | 列出可用 CSV 文件 |
| `POST` | `/instances/{id}/csv-replay` | 配置 CSV 多测点同步回放 |
| `POST` | `/instances/{id}/batch-replay` | 批量回放 metrics_snapshot |
| `GET` | `/instances/{id}/metrics` | 获取 metrics 快照 |

### 4.3 微电网 API (服务模式)

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/microgrid/{id}/topology` | 获取微电网拓扑 |
| `PUT` | `/api/v1/microgrid/{id}/topology` | 保存微电网拓扑 (热加载到引擎) |
| `PUT` | `/api/v1/microgrid/{id}/device` | 更新设备参数 |
| `POST` | `/api/v1/microgrid/{id}/device` | 添加设备 (自动分配 IOABase) |
| `DELETE` | `/api/v1/microgrid/{id}/device/{devId}` | 删除设备 |
| `POST` | `/api/v1/microgrid/{id}/control/{devId}?closed=true` | 开关控制 |
| `GET` | `/api/v1/microgrid/{id}/dashboard` | 获取运行面板数据 |
| `GET` | `/api/v1/microgrid/{id}/points` | 获取测点列表 (含 can_toggle/local_mode) |
| `GET` | `/api/v1/microgrid/{id}/formulas` | 获取公式列表 |
| `POST` | `/api/v1/microgrid/{id}/formulas` | 添加公式 |
| `PUT` | `/api/v1/microgrid/{id}/formulas` | 更新公式 |
| `DELETE` | `/api/v1/microgrid/{id}/formulas/{id}` | 删除公式 |
| `GET` | `/api/v1/microgrid/{id}/export-xlsx` | 导出打包的 XLSX 点表 |

### 4.4 接口测试 API (Proxy)

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/api/v1/proxy` | 执行 HTTP 代理请求（发送请求并返回响应） |
| `GET` | `/api/v1/proxy/collections` | 获取所有接口集合/请求列表 |
| `POST` | `/api/v1/proxy/collections` | 创建/更新接口或文件夹（upsert） |
| `DELETE` | `/api/v1/proxy/collections/{id}` | 删除接口或文件夹 |
| `GET` | `/api/v1/proxy/environments` | 获取环境变量列表及当前激活环境 |
| `POST` | `/api/v1/proxy/environments` | 创建/更新环境变量组（upsert） |
| `POST` | `/api/v1/proxy/environments/{id}/activate` | 激活指定环境变量组 |
| `DELETE` | `/api/v1/proxy/environments/{id}` | 删除环境变量组 |
| `GET` | `/api/v1/proxy/export` | 导出全部代理配置（集合+环境） |

### 4.5 传统模式 API (`/api/`)

单实例模式下可用:

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/points` | 列出所有测点 |
| `POST` | `/api/points` | 批量更新测点值 (触发变化上送) |
| `GET` | `/api/points/{ioa}` | 获取单点 |
| `PUT` | `/api/points/{ioa}` | 更新单点值 (触发变化上送) |
| `PUT` | `/api/points/{ioa}/qds` | 更新品质描述 QDS |
| `GET` | `/api/status` | 服务状态 (运行时长/统计/客户端连接) |

---

## 5. MCP 工具

GridSim 内置 MCP Server，提供 38 个工具供 AI 代理直接调用。分为三组：

### 5.1 实例管理工具 (InstanceManager)

| 工具 | 参数 | 说明 |
|------|------|------|
| `list_instances` | — | 列出所有已配置的模拟器实例 |
| `get_instance` | `instance_id` | 获取单个实例的详细配置信息 |
| `create_instance` | `config` JSON | 创建新实例 |
| `update_instance` | `instance_id`, `config` JSON | 更新实例配置 |
| `delete_instance` | `instance_id` | 删除实例 |
| `start_instance` | `instance_id` | 启动实例 |
| `stop_instance` | `instance_id` | 停止实例 |
| `restart_instance` | `instance_id` | 重启实例 |
| `get_server_status` | — | 全局状态 (运行数/总数) |
| `list_files` | — | 列出 config 目录下的 .xlsx 点表文件 |
| `get_protocols` | — | 查询支持的协议 |

### 5.2 数据接口工具 (DataInterface)

| 工具 | 必填参数 | 可选参数 | 说明 |
|------|---------|---------|------|
| `list_points` | `instance_id` | — | 列出实例的所有测点及当前值 |
| `read_point` | `instance_id`, `ioa` | — | 读取单个测点值 |
| `read_points` | `instance_id` | `ioas[]` | 批量读取(不传ioas返回全部) |
| `write_point` | `instance_id`, `ioa` | `value`, `bool_value`, `int_value` | 写入单点值 |
| `write_points` | `instance_id`, `points[]` | — | **批量写入** 多个测点值(模拟同时上报) |
| `config_auto_change` | `instance_id`, `ioa`, `strategy`, `enabled` | `params` JSON | 配置测点自动变化策略 |
| `get_auto_change` | `instance_id`, `ioa` | — | 查看自动变化配置 |
| `delete_auto_change` | `instance_id`, `ioa` | — | 删除自动变化配置 |
| `export_auto_changes` | `instance_id` | — | 导出自变化配置 CSV |
| `import_auto_changes` | `instance_id`, `csv_content` | — | 导入自动变化配置 |
| `upload_csv` | `instance_id`, `csv_content` | — | 上传 CSV 时间序列文件 |
| `list_csv_files` | `instance_id` | — | 列出可用 CSV 回放文件 |
| `config_csv_replay` | `instance_id`, `csv_file`, `mappings[]` | `time_format`, `time_unit`, `csv_loop` | **CSV多测点同步回放** |
| `batch_config_auto_change` | `instance_id`, `ioas[]`, `strategy`, `enabled` | `params` JSON | 批量配置自动变化 |
| `update_qds` | `ioa` | `invalid`, `not_topical`, `substituted`, `overflow`, `blocked` | 更新品质描述 |
| `get_instance` | `instance_id` | — | 获取实例详情 (此工具在两组中均有) |

### 5.3 接口测试工具 (ProxyServer)

| 工具 | 必填参数 | 可选参数 | 说明 |
|------|---------|---------|------|
| `proxy_list_collections` | — | — | 获取所有 API 接口集合/请求列表 |
| `proxy_create_collection` | `name`, `type` | `method`, `url`, `headers`, `body`, `pre_script`, `test_script`, `parent_id` | 创建接口或文件夹 |
| `proxy_update_collection` | `id` | `name`, `method`, `url`, `headers`, `body`, `pre_script`, `test_script` | 修改接口配置 |
| `proxy_delete_collection` | `id` | — | 删除接口或文件夹 |
| `proxy_execute_request` | `method`, `url` | `headers`, `body`, `timeout` | 执行 HTTP 代理请求 |
| `proxy_list_environments` | — | — | 获取环境变量列表 |
| `proxy_create_environment` | `name`, `variables` | — | 创建环境变量组 |
| `proxy_update_environment` | `id` | `name`, `variables` | 更新环境变量 |
| `proxy_activate_environment` | `id` | — | 激活环境变量组 |
| `proxy_delete_environment` | `id` | — | 删除环境变量组 |

---

## 6. 自动变化策略 (11种)

通过 `config_auto_change` 工具或 REST API 配置，策略 goroutine 周期性计算并更新测点值，同时触发 IEC104 变化上送。

| 策略 | code | 参数 | 说明 |
|------|------|------|------|
| `increment` | 1 | `start_value`, `step`, `period_ms`, `max_value` | 递增: 每周期+=step, 到max_value归零 |
| `random` | 2 | `min_value`, `max_value_r`, `period_ms`, `decimal_places` | 随机: 在[min, max]内生成随机值 |
| `csv` | 3 | `csv_file`, `time_format`, `time_unit`, `csv_column_map`, `csv_loop` | CSV回放: 按CSV时间序列播放 |
| `max` | 4 | `linked_ioas` | 取大: 监控多个IOA, 值=它们的最大值 |
| `min` | 5 | `linked_ioas` | 取小: 监控多个IOA, 值=它们的最小值 |
| `soc` | 6 | `power_ioa`, `capacity`, `init_soc`, `soc_min`, `soc_max`, `integral_ms` | SOC计算: 基于功率积分计算荷电状态 |
| `energy` | 7 | `energy_power_ioa`, `init_energy`, `energy_period_ms`, `stat_type` | 电量: 基于功率积分计算累计电量 |
| `aofollow` | 8 | `follow_ao_ioa` | AO关联: 当指定AO被遥控时, 同步更新本点 |
| `apiupdate` | 9 | `api_init_value` | 接口更新: 仅允许HTTP API写入, 引擎不计算 |
| `manual` | 10 | — | 手动: 不自动计算, 需API置数 |
| `custom` | 11 | `custom_ioas`, `custom_formula`, `period_ms` | 自定义公式: 四则运算, 支持2~50个关联测点 |

**配置示例** (递增策略):
```json
{"strategy":"increment","enabled":true,"params":{"start_value":0,"step":1,"period_ms":1000,"max_value":100}}
```

**注意事项**:
- AO/DO 测点不支持自动变化策略
- 策略运行时写 `apiupdate` / `manual` 策略会阻止 API 置数 (返回 403)
- 停止实例时所有策略 goroutine 自动取消

---

## 7. 微电网仿真引擎

### 7.1 设备类型

| 类型 | 标识 | 测点(每组50 IOA) | 仿真行为 |
|------|------|------------------|---------|
| 光伏 | `pv` | 有功功率, 日发电量, 运行状态, 开关状态, 功率设定, 远程启机 | 按 RatedPowerKW 发电, 开关控制 |
| 储能 | `battery` | 电池SOC, 充放电功率, 运行状态, 开关状态, 功率设定, 远程启机 | SOC积分, 按调度充放电, AO设定值控制 |
| 负荷 | `load` | 有功功率, 运行状态, 开关状态, 功率设定, 遥控分合 | 按 LoadRatedKW 消耗, 开关控制 |
| 充电桩 | `charger` | 充电功率, 运行状态, 开关状态, 功率设定, 遥控分合 | 按 ChargerRatedKW 消耗, 开关控制 |

### 7.2 关口表 (固定IOA)

| 测点 | IOA | 类型 | 说明 |
|------|-----|------|------|
| 关口表_有功功率 | 1 | AI | 电网交换功率 (>0取电, <0送电) |
| 关口表_无功功率 | 2 | AI | 无功功率 (≈有功×0.15) |
| 关口表_电压 | 3 | AI | 母线电压 (固定标识) |
| 关口表_频率 | 4 | AI | 电网频率 (≈50Hz) |
| 关口表_运行状态 | 1001 | DI | 并网/离网 |
| 关口表_孤岛状态 | 1002 | DI | 孤岛标志 |

### 7.3 IOA 分配规则 (方案B)

- 关口表保留区: IOA 1-100
- 每个设备分配连续的 50 个 IOA (101-150, 151-200, ...)
- 设备自声明 `IOABase` 字段, 删除后释放的块可被后续设备复用
- 旧格式兼容: IOABase=0 时按数组位置 `101 + idx*50` fallback
- AI 区: base ~ base+9 | DI 区: base+10 ~ base+19 | AO 区: base+20 ~ base+29 | DO 区: base+30 ~ base+39 | 自定义区: base+40 ~ base+49

### 7.4 引擎工作流程 (每 Tick)

```
1. 读取 PV 发电功率 (按 RatedPowerKW × 天气曲线)
2. 读取 负荷/充电桩 功率 (按 RatedKW × 负载曲线)
3. 计算电池功率:
   a. 优先使用 AO 设定值 (遥控)
   b. 回退到时段调度 (10-15h 充电, 18-22h 放电)
4. 更新电池 SOC (功率积分)
5. 功率平衡: 关口功率 = 总负载 - 总发电
6. 额定容量限幅 (关口表 RatedCapacityKW)
7. 同步到 Store (writePt → SetValue + Publish)
8. 评估用户自定义公式
9. 记录历史快照
```

### 7.5 控制模式

| 模式 | 说明 |
|------|------|
| `remote` (远方) | 功率由 AO 遥调设定值控制 |
| `local` (本地) | 功率由引擎按时段调度策略自动计算 |

### 7.6 自定义公式

- 基于递归下降解析器, 支持 `+ - * / ( )`
- 引用测点: `{测点名}` 如 `{光伏1_有功功率}`
- 自动公式: 引擎自动创建 `关口功率 = (负载+充电+电池充电) - (光伏+电池放电)`
- 公式目标通常是 `关口表_有功功率`

### 7.7 Dashboard API 返回

```json
{
  "grid_power_kw": -46.5,
  "total_pv_kw": 100.0,
  "total_bat_kw": 23.4,
  "total_load_kw": 76.9,
  "total_charger_kw": 0,
  "pv_devices": [{ "id": "dev-1", "name": "光伏1", "power_kw": 100, "closed": true, "mode": "local" }],
  "battery_devices": [{ "id": "dev-2", "name": "储能1", "power_kw": -23.4, "soc": 65.2, "closed": true, "mode": "remote" }],
  "load_devices": [...],
  "charger_devices": [...]
}
```

---

## 8. Web 前端

### 8.1 页面

| 页面 | 路由 | 功能 |
|------|------|------|
| 配置管理 | `/config` | 实例 CRUD, 启停, 容量显示 |
| 实例详情 | `/detail/{id}` | 测点实时刷新, 置数, 11种策略, CSV回放 |
| 趋势对比 | `/trend/{id}` | ECharts 实时曲线, 缩放拖拽, 时间范围选择 |
| 运行监控 | `/monitor` | 实例运行状态总览 |
| 微电网编辑 | `/microgrid/{id}` | 拓扑编辑, 设备增删改, IOA可视化, 公式管理 |

### 8.2 微电网编辑器功能

- 关口表配置: 额定容量(kW), 母线名称, 母线电压(kV), 孤岛模式开关
- 设备管理: 添加/编辑/删除 PV/电池/负荷/充电桩
- 设备参数编辑: 额定功率, 容量, SOC限值, 控制模式等
- IOA 冲突检测: 自动校验 + 前端冲突提示
- SVG 一次接线图: 开关可视化 + 点击控制
- 自定义测点: 设备支持 AI/DI/DO/AO 自定义测点
- 公式管理: 添加/编辑/删除自定义公式
- 导出拓扑 JSON / 导出 XLSX 点表
- 导入拓扑 JSON

---

## 9. 关键设计说明

### 变化上送机制

```
writePt() 或 API 置数
    → store.SetValue(ioa, val)  // 写入 Store, 设置 Changed=true
    → pub.Publish(point)        // 发送到 IEC104 publishCh(容量1024)
    → publishLoop goroutine     // 从 channel 取出, 调用 sendSpontaneous()
    → IEC104 服务端发送 COT=3 ASDU 到客户端
```

- 微电网引擎: `syncStoreLocked()` 调用 `writePt()` → `SetValue` + `Publish`
- 自动变化策略引擎: `sr.publisher.Publish(p)` 在策略计算后调用
- HTTP API: 置数后通过 `engine.pub.Publish()` 触发
- IEC104 遥控/遥调: 在 `handleSingleCommand` / `handleSetpointCommand` 中 `Publish`

### API 置数权限

- 普通策略: 可通过 API 置数 (API 写入覆盖策略值)
- `apiupdate` / `manual` 策略: API 置数返回 403 Forbidden

### AO 关联机制

当 AO 测点被遥控(设定值变更)时:
1. `AOFollowFn(aoIOA)` 被调用
2. 查找所有配置了 `aofollow` 策略且 `follow_ao_ioa` 匹配的测点
3. 同步更新这些测点的值为 AO 的当前值
4. 触发变化上送
