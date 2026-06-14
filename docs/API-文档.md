# GridSim API 完整使用文档

> 版本: v3.0.1 | 更新日期: 2026-06-14  
> Base URL: `http://localhost:8989`

---

## 目录

1. [概述](#1-概述)
2. [认证](#2-认证)
3. [全局管理接口](#3-全局管理接口-apiv1)
4. [实例管理接口](#4-实例管理接口-apiv1instances)
5. [测点数据接口 (Server Mode)](#5-测点数据接口-server-mode)
6. [自动变化策略接口](#6-自动变化策略接口)
7. [CSV 文件与回放接口](#7-csv-文件与回放接口)
8. [批量回放与指标接口](#8-批量回放与指标接口)
9. [接口测试 (Proxy) 接口](#9-接口测试-proxy-接口)
10. [微电网 (Microgrid) 接口](#10-微电网-microgrid-接口)
11. [点表接口 (Legacy Mode)](#11-点表接口-legacy-mode)
12. [数据类型](#12-数据类型)
13. [错误处理](#13-错误处理)
14. [AI 友好接口 (v3.0.1 新增)](#14-ai-友好接口-v301-新增)

---

## 1. 概述

GridSim 提供两种运行模式，对应不同的 API 前缀：

| 模式 | 启动方式 | API 前缀 |
|------|----------|----------|
| **Server Mode** | `./bin/gridsim serve --http :8989` | `/api/v1/...` |
| **Legacy Mode** | `./bin/gridsim -p 2404 -c point.xlsx -H :8080` | `/api/...` |

Server Mode 支持多实例管理、微电网仿真、接口测试等全部功能；Legacy Mode 仅支持单实例的点表读写。

---

## 2. 认证

### POST `/api/v1/auth/login`

用户登录，返回 JWT Token。

**请求体：**
```json
{
  "username": "admin",
  "password": "admin123"
}
```

**响应 200：**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "user": {
    "id": "u-001",
    "username": "admin",
    "role": "admin"
  }
}
```

**错误 401：**
```json
{ "error": "invalid credentials" }
```

> Token 用于后续需要认证的请求，在 Header 中携带 `Authorization: Bearer <token>`。

---

## 3. 全局管理接口 `/api/v1`

### 3.1 获取全局状态

```
GET /api/v1/status
```

**响应 200：**
```json
{
  "version": "3.0.0",
  "git_commit": "abc1234",
  "git_branch": "main",
  "mode": "serve",
  "configured": 5,
  "running": 3,
  "stopped": 2,
  "max": 1000
}
```

### 3.2 获取已上传文件列表

```
GET /api/v1/files
```

返回 config 目录下所有 `.xlsx` 点表文件。

**响应 200：**
```json
{
  "files": [
    {
      "name": "point.xlsx",
      "size": 12345,
      "modtime": "2026-05-12T10:30:00Z"
    }
  ]
}
```

### 3.3 上传点表文件

```
POST /api/v1/upload
Content-Type: multipart/form-data

file: <选择 .xlsx/.csv 文件>
```

- 最大文件大小：10MB
- 允许格式：`.xlsx`, `.xls`, `.csv`
- 禁止覆盖同名文件

**响应 200：**
```json
{
  "status": "uploaded",
  "filename": "my_points.xlsx"
}
```

### 3.4 查询支持的协议

```
GET /api/v1/protocols
```

**响应 200：**
```json
{
  "protocols": ["iec104", "modbus_tcp", "microgrid"]
}
```

---

## 4. 实例管理接口 `/api/v1/instances`

### 4.1 获取实例列表

```
GET /api/v1/instances
```

**响应 200：**
```json
{
  "instances": [
    {
      "id": "a1b2c3d4e5f6",
      "name": "变电站A",
      "iec104_port": 2404,
      "xlsx_file": "samples/point.xlsx",
      "enabled": false,
      "http_enabled": true,
      "http_port": 8081,
      "protocol": "iec104",
      "status": "running",
      "stats": {
        "uptime_seconds": 3600,
        "total_points": 7,
        "client_connected": true,
        "interrogations": 5,
        "controls": 12,
        "spontaneous": 88
      }
    }
  ]
}
```

### 4.2 创建实例

```
POST /api/v1/instances
Content-Type: application/json
```

**IEC 104 实例：**
```json
{
  "name": "变电站A",
  "iec104_port": 2404,
  "xlsx_file": "samples/point.xlsx",
  "http_enabled": true,
  "http_port": 8081
}
```

**Modbus TCP 实例：**
```json
{
  "name": "Modbus设备",
  "iec104_port": 2502,
  "xlsx_file": "samples/modbus_points.xlsx",
  "protocol": "modbus_tcp",
  "modbus_config": {
    "port": 2502,
    "byte_order": "ABCD",
    "slave_id": 1
  }
}
```

**微电网实例：**
```json
{
  "name": "微电网仿真",
  "iec104_port": 2404,
  "protocol": "microgrid",
  "microgrid_config": {
    "topology_json": "{...}",
    "tick_ms": 1000,
    "speed_factor": 1.0
  }
}
```

**响应 201：** 返回创建后的完整实例配置（含自动生成的 `id`）。

### 4.3 获取实例详情

```
GET /api/v1/instances/{id}
```

**响应 200：** 返回实例配置 + 运行状态（同列表中单项格式）。

### 4.4 更新实例

```
PUT /api/v1/instances/{id}
Content-Type: application/json
```

支持部分更新 — 只需传需要修改的字段。如正在运行，会先停止再更新配置。

```json
{
  "name": "变电站A-修改",
  "iec104_port": 2405
}
```

**响应 200：** 返回更新后的完整配置。

### 4.5 删除实例

```
DELETE /api/v1/instances/{id}
```

如正在运行，会先停止再删除。

**响应 200：**
```json
{ "status": "deleted" }
```

### 4.6 启动实例

```
POST /api/v1/instances/{id}/start
```

**响应 200：**
```json
{ "status": "ok", "id": "a1b2c3d4e5f6" }
```

### 4.7 停止实例

```
POST /api/v1/instances/{id}/stop
```

**响应 200：**
```json
{ "status": "ok", "id": "a1b2c3d4e5f6" }
```

### 4.8 重启实例

```
POST /api/v1/instances/{id}/restart
```

如未运行，则直接启动。

**响应 200：**
```json
{ "status": "ok", "id": "a1b2c3d4e5f6" }
```

---

## 5. 测点数据接口 (Server Mode)

以下接口需要实例处于 **running** 状态。

### 5.1 获取所有测点快照

```
GET /api/v1/instances/{id}/points
```

返回所有测点的实时快照，按 AI 优先 + IOA 升序排列。

**响应 200：**
```json
{
  "points": [
    {
      "ioa": 16385,
      "name": "母线电压",
      "point_type": "AI",
      "value": 100.5,
      "bool_value": false,
      "int_value": 0,
      "updated_at": "2026-05-12T02:30:00.000Z",
      "unit": "",
      "function_code": 0,
      "register_address": 0,
      "byte_order": ""
    }
  ],
  "refreshed_at": "2026-05-12T02:30:00.123Z"
}
```

### 5.2 获取单个测点快照

```
GET /api/v1/instances/{id}/points/{ioa}
```

**响应 200：** 单个 PointSnapshot 对象。

### 5.3 置数（写入点值）

```
PUT /api/v1/instances/{id}/points/{ioa}
Content-Type: application/json
```

根据测点类型使用不同字段：

**AI 遥测：**
```json
{ "value": 235.5 }
```

**DI 遥信：**
```json
{ "bool_value": true }
```
也支持 `{ "value": 1 }` (非零 = true)。

**PI 遥脉：**
```json
{ "int_value": 42 }
```
也支持 `{ "value": 42 }`。

**响应 200：**
```json
{
  "success": true,
  "ioa": 16385,
  "changed": true
}
```

> 置数后会触发 IEC104 变化上送 (COT=3)。AO/DO 类型不支持直接置数。

**限制：** 若测点已配置自动变化策略且策略禁止 API 写入，返回 403。

### 5.4 批量置数

```
POST /api/v1/instances/{id}/points/batch
Content-Type: application/json
```

一次请求写入多个测点，确保数据一致性。

```json
{
  "points": [
    { "ioa": 16385, "value": 999.99 },
    { "ioa": 5, "bool_value": false },
    { "ioa": 10, "int_value": 100 }
  ]
}
```

**响应 200：**
```json
{
  "success": true,
  "results": [
    { "ioa": 16385, "success": true, "changed": true },
    { "ioa": 5, "success": true, "changed": true },
    { "ioa": 10, "success": true, "changed": true }
  ],
  "total": 3,
  "succeeded": 3,
  "failed": 0
}
```

### 5.5 批量读取

```
GET /api/v1/instances/{id}/points/batch?ioas=16385,16386,16387
```

一次请求读取多个指定 IOA 的快照。用逗号分隔 IOA。

**响应 200：**
```json
{
  "points": [ ... ],
  "refreshed_at": "2026-05-12T02:30:00.123Z"
}
```

### 5.6 导出测点 CSV

```
GET /api/v1/instances/{id}/points/export
```

下载所有测点实时数据为 CSV 文件。按 AI 优先 + IOA 升序排列。

**响应 200：** `Content-Type: text/csv; charset=utf-8`，带 BOM 头。

CSV 列：`信息体地址, 测点名称, 测点类型, 实时值, 测点值更新时间`

---

## 6. 自动变化策略接口

### 6.1 获取测点自动变化配置

```
GET /api/v1/instances/{id}/points/auto-change/{ioa}
```

**响应 200：**
```json
{
  "ioa": 16385,
  "strategy": "increment",
  "enabled": true,
  "params": {
    "start_value": 0,
    "step": 1,
    "period_ms": 1000,
    "max_value": 100
  },
  "updated_at": "2026-05-12T02:30:00Z"
}
```

**响应 404：** `{ "error": "no auto-change config" }`

### 6.2 配置自动变化

```
PUT /api/v1/instances/{id}/points/auto-change/{ioa}
Content-Type: application/json
```

```json
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

**支持的策略类型与参数：**

| 策略 | 类型名 | 参数说明 |
|------|--------|----------|
| 递增 | `increment` | `start_value`, `step`, `period_ms`, `max_value` |
| 随机 | `random` | `min_value`, `max_value_r`, `period_ms`, `decimal_places` |
| CSV回放 | `csv` | `csv_file`, `time_format`, `time_unit`, `csv_column_map`, `csv_loop` |
| 取大 | `max` | `para_a` (IOA列表，分号分隔), `para_b` (关联IOA) |
| 取小 | `min` | 同 max |
| SOC计算 | `soc` | `init_soc`, `rated_cap`, `power_ioa`, `integral_ms` |
| 电量统计 | `energy` | `init_energy`, `stat_type`(0充电/1放电), `energy_power_ioa`, `energy_period_ms` |
| AO关联 | `aofollow` | `follow_ao_ioa` |
| 接口更新 | `apiupdate` | `api_init_value` |
| 手动 | `manual` | 无参数，不自动计算 |
| 自定义公式 | `custom` | `custom_ioas`(至少2个), `custom_formula`, `period_ms` |

> `period_ms` 最小值 100ms。`custom` 策略关联测点最多 50 个。AO/DO 不支持自动变化。

**响应 200：**
```json
{ "success": true, "ioa": 16385 }
```

### 6.3 删除自动变化配置

```
DELETE /api/v1/instances/{id}/points/auto-change/{ioa}
```

**响应 200：**
```json
{ "success": true }
```

### 6.4 批量配置自动变化

```
PUT /api/v1/instances/{id}/points/auto-change/batch
Content-Type: application/json
```

为多个 IOA 应用同一策略配置：

```json
{
  "ioas": [16385, 16386, 16387],
  "config": {
    "strategy": "increment",
    "enabled": true,
    "params": { "start_value": 0, "step": 1, "period_ms": 1000, "max_value": 100 }
  }
}
```

**响应 200：**
```json
{
  "success": true,
  "total": 3,
  "ok": 3,
  "failed": 0,
  "errors": []
}
```

### 6.5 导出自动变化配置

```
GET /api/v1/instances/{id}/points/auto-change/export
```

下载所有自动变化配置为 CSV 文件（带 BOM 头，Excel 兼容）。

**CSV 列：** `信息体地址, 测点名称, 自动变化模式, A~G`

策略代码映射：1=递增 2=随机 3=CSV 4=取大 5=取小 6=SOC 7=电量 8=AO关联 9=接口更新 10=手动

### 6.6 导入自动变化配置

```
POST /api/v1/instances/{id}/points/auto-change/import
Content-Type: multipart/form-data

file: <CSV 文件>
```

上传 CSV 文件批量导入自动变化配置。

**响应 200：**
```json
{
  "success": true,
  "total": 10,
  "applied": 8,
  "skipped": 2
}
```

---

## 7. CSV 文件与回放接口

### 7.1 上传 CSV 文件

```
POST /api/v1/instances/{id}/upload-csv
Content-Type: multipart/form-data

file: <CSV 文件>
```

为该实例上传 CSV 时间序列文件，存储到实例私有目录。

**响应 200：**
```json
{
  "success": true,
  "filename": "replay_data.csv",
  "path": "csv/a1b2c3d4e5f6/replay_data.csv"
}
```

### 7.2 列出 CSV 文件

```
GET /api/v1/instances/{id}/csv-files
```

列出共享目录和实例私有目录中的 CSV 文件，按修改时间倒序。

**响应 200：**
```json
{
  "files": [
    {
      "name": "replay_data.csv",
      "size": 1024,
      "modtime": "2026-05-12T10:00:00Z",
      "shared": false
    }
  ]
}
```

### 7.3 读取 CSV 文件内容（前100行）

```
GET /api/v1/instances/{id}/csv-content/{filename}
```

返回 CSV 文件的前 100 行内容，用于预览和列映射。

**响应 200：**
```json
{
  "filename": "replay_data.csv",
  "content": "time,voltage,current\n0,220.0,5.2\n1000,221.5,5.3"
}
```

### 7.4 配置 CSV 多测点同步回放

```
POST /api/v1/instances/{id}/csv-replay
Content-Type: application/json
```

一次调用为多个测点配置 CSV 回放策略，所有测点共享同一时间基准。

```json
{
  "csv_file": "replay_data.csv",
  "time_format": "relative",
  "time_unit": "ms",
  "csv_loop": true,
  "mappings": [
    { "column": 1, "ioa": 16385 },
    { "column": 2, "ioa": 16386 },
    { "column": 3, "ioa": 16387 }
  ]
}
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `csv_file` | string | 是 | 已上传的 CSV 文件名 |
| `time_format` | string | 否 | `relative`(默认) / `absolute` |
| `time_unit` | string | 否 | `ms`(默认) / `s`，仅 relative 有效 |
| `csv_loop` | bool | 否 | 是否循环播放，默认 true |
| `mappings` | array | 是 | 列到测点映射，最多 10 个 |

**响应 200：**
```json
{
  "success": true,
  "total": 3,
  "succeeded": 3,
  "failed": 0,
  "results": [
    { "ioa": 16385, "success": true },
    { "ioa": 16386, "success": true },
    { "ioa": 16387, "success": true }
  ]
}
```

---

## 8. 批量回放与指标接口

### 8.1 批量 CSV 回放

```
POST /api/v1/instances/{id}/batch-replay
Content-Type: application/json
```

按顺序自动播放多个 CSV 文件，适用于自动化测试场景。

```json
{
  "csv_files": ["test1.csv", "test2.csv", "test3.csv"],
  "sequential": true,
  "auto_cleanup": true,
  "pause_between_files_s": 5,
  "on_error": "continue",
  "loop": false,
  "mappings": [
    { "column": 1, "ioa": 16385 },
    { "column": 2, "ioa": 16386 }
  ]
}
```

| 参数 | 说明 |
|------|------|
| `csv_files` | CSV 文件列表 |
| `auto_cleanup` | 播放前自动清理旧的自动变化配置 |
| `pause_between_files_s` | 文件间暂停秒数 |
| `on_error` | `continue`(继续) / `stop`(停止) |
| `mappings` | 列到测点映射 |

**响应 200：**
```json
{
  "batch_id": "batch_a1b2c3d4_1",
  "status": "running",
  "total": 3,
  "completed": 0,
  "failed": 0
}
```

### 8.2 查询批量回放进度

```
GET /api/v1/instances/{id}/batch-replay/{batch_id}
```

**响应 200：**
```json
{
  "batch_id": "batch_a1b2c3d4_1",
  "status": "done",
  "total": 3,
  "completed": 3,
  "failed": 0,
  "per_file_results": {
    "test1.csv": { "status": "done", "metrics_snapshot": {...} },
    "test2.csv": { "status": "done", "metrics_snapshot": {...} },
    "test3.csv": { "status": "done", "metrics_snapshot": {...} }
  }
}
```

### 8.3 获取指标数据

```
GET /api/v1/instances/{id}/metrics
```

返回关键测点的实时指标（默认读取 IOA 16385/16386/16387）。

**响应 200：**
```json
{
  "p_edc": 220.5,
  "count": 100,
  "cmd": 1.0,
  "pulse_status": "INACTIVE"
}
```

---

## 9. 接口测试 (Proxy) 接口

内置 Postman 风格的接口测试工具。

### 9.1 执行 HTTP 代理请求

```
POST /api/v1/proxy
Content-Type: application/json
```

```json
{
  "method": "GET",
  "url": "https://api.example.com/data",
  "headers": { "Authorization": "Bearer token123" },
  "body": "",
  "timeout": 30
}
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `method` | string | 否 | HTTP 方法，默认 GET |
| `url` | string | 是 | 目标 URL |
| `headers` | object | 否 | 自定义请求头 |
| `body` | string | 否 | 请求体 |
| `timeout` | int | 否 | 超时秒数 1~120，默认 30 |

> URL 中的 `{{变量名}}` 会被当前激活的环境变量自动替换。

**响应 200：**
```json
{
  "status": 200,
  "status_text": "OK",
  "headers": { "Content-Type": "application/json" },
  "body": "{\"data\": \"...\"}",
  "time_ms": 123,
  "size": 456
}
```

### 9.2 获取接口集合列表

```
GET /api/v1/proxy/collections
```

**响应 200：**
```json
{
  "collections": [
    {
      "id": "req-xxx",
      "name": "电力系统API",
      "type": "folder",
      "children": [
        {
          "id": "req-yyy",
          "name": "获取实时数据",
          "type": "request",
          "method": "GET",
          "url": "{{base_url}}/api/realtime",
          "headers": { "Content-Type": "application/json" },
          "body": "",
          "pre_script": "",
          "test_script": ""
        }
      ]
    }
  ]
}
```

### 9.3 创建/更新接口集合

```
POST /api/v1/proxy/collections
Content-Type: application/json
```

Upsert 操作 — 存在相同 ID 则更新，否则创建。

```json
{
  "id": "req-xxx",
  "name": "获取实时数据",
  "type": "request",
  "method": "GET",
  "url": "{{base_url}}/api/realtime",
  "headers": { "Content-Type": "application/json" },
  "body": "",
  "pre_script": "// 前置脚本",
  "test_script": "// 后置脚本",
  "children": []
}
```

### 9.4 删除接口集合

```
DELETE /api/v1/proxy/collections/{id}
```

**响应 200：** `{ "status": "deleted" }`

### 9.5 获取环境变量列表

```
GET /api/v1/proxy/environments
```

**响应 200：**
```json
{
  "environments": [
    {
      "id": "env-xxx",
      "name": "测试环境",
      "variables": {
        "base_url": "http://localhost:8989",
        "token": "dev-token"
      }
    }
  ],
  "active_id": "env-xxx"
}
```

### 9.6 创建/更新环境变量

```
POST /api/v1/proxy/environments
Content-Type: application/json
```

Upsert 操作。

```json
{
  "id": "env-xxx",
  "name": "测试环境",
  "variables": {
    "base_url": "http://localhost:8989",
    "token": "dev-token"
  }
}
```

### 9.7 激活环境

```
POST /api/v1/proxy/environments/{id}/activate
```

**响应 200：** `{ "status": "ok" }`

### 9.8 删除环境

```
DELETE /api/v1/proxy/environments/{id}
```

**响应 200：** `{ "status": "deleted" }`

### 9.9 导出全部接口测试配置

```
GET /api/v1/proxy/export
```

导出所有集合和环境变量为 JSON。

**响应 200：**
```json
{
  "version": "gridsim-proxy-export-v1",
  "exported_at": "2026-06-10T10:00:00Z",
  "collections": [...],
  "environments": [...],
  "active_env_id": "env-xxx"
}
```

---

## 10. 微电网 (Microgrid) 接口

微电网仿真功能，支持光伏(PV)、储能(Battery)、负荷(Load)、充电桩(Charger) 四种设备。

### 10.1 获取/更新拓扑

```
GET /api/v1/microgrid/{id}/topology
PUT /api/v1/microgrid/{id}/topology
```

**GET 响应 200：**
```json
{
  "bus_name": "10kV 母线",
  "bus_voltage_kv": 10,
  "grid_meter": {
    "rated_capacity_kw": 500,
    "island_mode": false
  },
  "devices": [...],
  "formulas": [...]
}
```

**PUT 请求体：** 完整的 Topology JSON。

### 10.2 设备增删改

```
POST /api/v1/microgrid/{id}/device
PUT /api/v1/microgrid/{id}/device
DELETE /api/v1/microgrid/{id}/device/{devId}
```

**POST 创建设备：**
```json
{
  "type": "pv",
  "name": "光伏1",
  "params": {
    "rated_power_kw": 100,
    "efficiency": 0.95
  },
  "switch": { "name": "QF1", "closed": true, "controllable": true },
  "control_mode": "local",
  "strategy": {
    "type": "increment",
    "enabled": true,
    "params": { "start_value": 0, "step": 1, "period_ms": 1000, "max_value": 100 }
  }
}
```

系统自动分配 `id` 和 `ioa_base`。

**设备类型与参数：**

| 类型 | type | 专有参数 |
|------|------|----------|
| 光伏 | `pv` | `rated_power_kw`, `efficiency` |
| 储能 | `battery` | `capacity_kwh`, `rated_power_kw_b`, `init_soc`, `soc_min`, `soc_max`, `eff` |
| 负荷 | `load` | `load_rated_kw`, `power_factor` |
| 充电桩 | `charger` | `charger_rated_kw`, `charger_eff` |

### 10.3 开关控制

```
POST /api/v1/microgrid/{id}/control/{devId}?closed=true
```

控制设备开关的开合状态。

**响应 200：**
```json
{ "status": "ok", "closed": true }
```

### 10.4 获取仪表盘数据

```
GET /api/v1/microgrid/{id}/dashboard
```

**响应 200（运行中）：**
```json
{
  "status": "running",
  "device_count": 4,
  "total_generation_kw": 150.5,
  "total_load_kw": 80.2,
  "grid_power_kw": -70.3
}
```

**响应 200（已停止）：**
```json
{
  "status": "stopped",
  "device_count": 4,
  "total_generation_kw": 0,
  "total_load_kw": 0,
  "grid_power_kw": 0
}
```

### 10.5 获取测点列表

```
GET /api/v1/microgrid/{id}/points
```

返回微电网所有测点，含 `can_toggle`(可切换本地/远方) 和 `local_mode`(当前是否本地策略) 标记。

**响应 200：**
```json
{
  "points": [
    {
      "ioa": 16385,
      "name": "光伏1_有功功率",
      "point_type": "AI",
      "value": 95.5,
      "unit": "kW",
      "can_toggle": true,
      "local_mode": true
    }
  ]
}
```

### 10.6 公式规则管理

```
GET /api/v1/microgrid/{id}/formulas
POST /api/v1/microgrid/{id}/formulas
PUT /api/v1/microgrid/{id}/formulas
DELETE /api/v1/microgrid/{id}/formulas/{formulaID}
```

**POST 创建公式：**
```json
{
  "name": "功率平衡",
  "target": "关口表_有功功率",
  "expression": "{Battery_Power} + {Load_Power}",
  "enabled": true
}
```

系统自动分配 `id`。

### 10.7 导出点表 XLSX

```
GET /api/v1/microgrid/{id}/export-xlsx
```

下载 ZIP 压缩包，包含按设备分组的独立 xlsx 文件和完整点表。

**响应 200：** `Content-Type: application/zip`

---

## 11. 点表接口 (Legacy Mode)

> Legacy Mode 使用 `./bin/gridsim` 直接启动，API 前缀为 `/api`。

### 11.1 获取所有点表

```
GET /api/points
```

**响应 200：**
```json
{
  "points": [
    {
      "ioa": 16385,
      "name": "母线电压",
      "value_type": "FLOAT",
      "point_type": "AI",
      "value": 100.5,
      "bool_value": false,
      "int_value": 0,
      "efficient": 1,
      "base_value": 100.5,
      "qds": {
        "invalid": false,
        "not_topical": false,
        "substituted": false,
        "overflow": false,
        "blocked": false
      },
      "alias": "",
      "timestamp": "2026-05-12T02:30:00Z"
    }
  ]
}
```

### 11.2 获取单个点

```
GET /api/points/{ioa}
```

### 11.3 更新遥测/遥调点值 (AI/AO)

```
PUT /api/points/{ioa}
Content-Type: application/json

{ "value": 235.5 }
```

### 11.4 更新遥信/遥控点值 (DI/DO)

```
PUT /api/points/{ioa}
Content-Type: application/json

{ "bool_value": true }
```

### 11.5 更新遥脉点值 (PI)

```
PUT /api/points/{ioa}
Content-Type: application/json

{ "int_value": 42 }
```

### 11.6 批量更新点值

```
POST /api/points
Content-Type: application/json
```

```json
{
  "points": [
    { "ioa": 16385, "value": 999.99 },
    { "ioa": 5, "bool_value": false },
    { "ioa": 10, "int_value": 100 }
  ]
}
```

**响应 200：**
```json
{
  "success": true,
  "updated": 3,
  "failed": 0,
  "details": [
    { "ioa": 16385, "success": true },
    { "ioa": 5, "success": true },
    { "ioa": 10, "success": true }
  ]
}
```

### 11.7 更新品质描述 (QDS)

```
PUT /api/points/{ioa}/qds
Content-Type: application/json
```

```json
{
  "invalid": true,
  "blocked": true
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `invalid` | bool | 无效 |
| `not_topical` | bool | 非当前 |
| `substituted` | bool | 取代 |
| `overflow` | bool | 溢出 |
| `blocked` | bool | 闭锁 |

### 11.8 获取服务端状态

```
GET /api/status
```

**响应 200：**
```json
{
  "connected": true,
  "client_addr": "192.168.1.100:51234",
  "uptime": 3600,
  "interrog": 5,
  "control": 12,
  "spont": 88
}
```

---

## 12. 数据类型

### InstanceConfig (实例配置)

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | string | 否 | 自动生成，12位 hex |
| `name` | string | 是 | 实例名称 |
| `iec104_port` | int | 是 | IEC104 端口号 (1-65535) |
| `xlsx_file` | string | 条件 | 点表 xlsx 文件名（microgrid 协议可选） |
| `enabled` | bool | 否 | 是否启用 |
| `http_enabled` | bool | 否 | 是否启用实例级 HTTP |
| `http_port` | int | 条件 | 实例 HTTP 端口（http_enabled=true 时必填） |
| `protocol` | string | 否 | `iec104`(默认) / `modbus_tcp` / `microgrid` |
| `modbus_config` | object | 否 | Modbus 附加配置 |
| `microgrid_config` | object | 否 | 微电网附加配置 |

### InstanceState (实例运行状态)

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | string | 实例ID |
| `name` | string | 实例名称 |
| `iec104_port` | int | IEC104 端口 |
| `xlsx_file` | string | 点表文件 |
| `protocol` | string | 协议类型 |
| `status` | string | `running` / `stopped` / `error` |
| `stats.uptime_seconds` | int64 | 运行时长（秒） |
| `stats.total_points` | int | 测点总数 |
| `stats.client_connected` | bool | 客户端是否已连接 |
| `stats.interrogations` | int64 | 总召次数 |
| `stats.controls` | int64 | 遥控次数 |
| `stats.spontaneous` | int64 | 变化上送次数 |

### Point (测点 - Legacy)

| 字段 | 类型 | 说明 |
|------|------|------|
| `ioa` | uint32 | IOA 地址 |
| `name` | string | 点名 |
| `value_type` | string | `FLOAT` / `DOUBLE` / `INT` / `BIT` |
| `point_type` | string | `AI` / `DI` / `PI` / `DO` / `AO` |
| `value` | float64 | 浮点值（AI/AO） |
| `bool_value` | bool | 布尔值（DI/DO） |
| `int_value` | int32 | 整型值（PI） |
| `efficient` | float64 | 系数 |
| `base_value` | float64 | 基值 |
| `qds` | object | 质量描述 |
| `alias` | string | 别名 |
| `timestamp` | string | 更新时间 |

### PointSnapshot (测点快照 - Server Mode)

| 字段 | 类型 | 说明 |
|------|------|------|
| `ioa` | uint32 | IOA 地址 |
| `name` | string | 点名 |
| `point_type` | string | 测点类型 |
| `value` | float64 | 浮点值 |
| `bool_value` | bool | 布尔值 |
| `int_value` | int32 | 整型值 |
| `updated_at` | string | 更新时间 |
| `unit` | string | 单位 |
| `function_code` | uint8 | Modbus 功能码（仅 Modbus） |
| `register_address` | uint16 | Modbus 寄存器地址（仅 Modbus） |
| `byte_order` | string | 字节序（仅 Modbus） |

### AutoChangeConfig (自动变化配置)

| 字段 | 类型 | 说明 |
|------|------|------|
| `ioa` | uint32 | 测点 IOA |
| `strategy` | string | 策略类型 |
| `enabled` | bool | 是否启用 |
| `params` | object | 策略参数（见各策略说明） |
| `updated_at` | string | 最后更新时间 |

---

## 13. 错误处理

> **v3.0.1 更新**: 错误响应升级为结构化格式，包含错误码、修复建议和候选值。

所有错误响应使用统一的 JSON 格式：

```json
{
  "error": {
    "code": "INSTANCE_NOT_RUNNING",
    "message": "instance is not running",
    "hint": "start the instance first using POST /api/v1/instances/{id}/start",
    "candidates": ["instance-001", "instance-002"]
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `error.code` | string | 大写蛇形错误码（如 `INSTANCE_NOT_FOUND`） |
| `error.message` | string | 人类可读的错误描述 |
| `error.hint` | string | 修复建议（可选） |
| `error.candidates` | string[] | 候选值列表，辅助 AI Agent 自动修复（可选） |

> **兼容性**: v3.0.1 仍兼容旧格式 `{ "error": "错误描述" }`，前端自动适配两种格式。

### HTTP 状态码说明

| 状态码 | 含义 |
|--------|------|
| 200 | 成功 |
| 201 | 创建成功 |
| 400 | 请求参数错误（缺少字段、JSON 格式错误） |
| 401 | 认证失败（无效凭据） |
| 403 | 禁止操作（策略冲突） |
| 404 | 资源不存在 |
| 405 | 方法不允许 |
| 409 | 冲突（端口已使用、文件已存在等） |
| 500 | 服务端内部错误 |

### 常见错误

| 场景 | 错误信息 | 解决方案 |
|------|----------|----------|
| 实例未运行时操作测点 | `instance not running` | 先启动实例 |
| 置数被策略阻止 | `该点已配置自动变化策略，不允许通过接口写入` | 删除策略或使用 manual 策略 |
| AO/DO 置数 | `AO/DO does not support set-value` | AO/DO 由遥控/遥调触发 |
| 端口冲突 | `port 2404 already in use` | 更换端口 |
| IOA 不存在 | `point not found` | 检查点表配置 |
| 自定义公式参数不足 | `custom formula requires at least 2 associated IOAs` | 至少关联 2 个测点 |

---

## 14. AI 友好接口 (v3.0.1 新增)

v3.0.1 新增一系列面向 AI Agent 的接口，让 AI 助手更高效地自主操作 GridSim。

### 14.1 OpenAPI 3.0 规范

```
GET /openapi.json
```

返回完整的 OpenAPI 3.0 JSON 文档，包含所有路径、请求/响应 Schema、错误码定义。AI Agent 可据此自动发现和理解接口。

**响应示例:**

```json
{
  "openapi": "3.0.0",
  "info": { "title": "GridSim API", "version": "3.0.1" },
  "paths": { ... },
  "components": { "schemas": { ... } }
}
```

### 14.2 全局统一状态快照

```
GET /api/v1/state
```

一次调用获取全部实例 + 测点运行状态，避免 N 次轮询。适用于 AI Agent 快速感知全局。

**响应示例:**

```json
{
  "instances": [
    {
      "id": "inst-001",
      "name": "变电站A",
      "status": "running",
      "port": 2404,
      "protocol": "iec104",
      "point_count": 100,
      "client_connected": true
    }
  ],
  "total_instances": 1,
  "running_instances": 1
}
```

### 14.3 SSE 事件推送

```
GET /api/v1/events
```

Server-Sent Events 流，实时推送实例状态变化、测点变化事件。

**事件类型:**

| event | 说明 |
|-------|------|
| `instance.started` | 实例启动 |
| `instance.stopped` | 实例停止 |
| `point.changed` | 测点值变化 |

**使用示例:**

```bash
curl -N http://localhost:8989/api/v1/events
```

### 14.4 场景录制

```
GET  /api/v1/recordings          # 获取录制列表
POST /api/v1/recordings          # 开始/停止录制
POST /api/v1/recordings/{id}/play # 回放录制
```

记录操作序列（创建实例、启停、置数、策略配置等），支持回放复现。适用于测试场景固化与自动化回归。

**开始录制:**

```bash
curl -X POST http://localhost:8989/api/v1/recordings \
  -H 'Content-Type: application/json' \
  -d '{"action": "start", "name": "test-scenario-1"}'
```

### 14.5 幂等性支持

所有写操作（POST/PUT/DELETE）支持 `Idempotency-Key` 请求头：

```bash
curl -X POST http://localhost:8989/api/v1/instances \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: my-unique-key-001' \
  -d '{"name": "变电站A", "port": 2404}'
```

- 相同 `Idempotency-Key` 的请求在 24h 内返回缓存的首次响应
- AI Agent 可安全重试网络失败的操作，避免重复创建
