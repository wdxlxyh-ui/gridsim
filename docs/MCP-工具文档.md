# GridSim MCP 工具完整使用文档

> 版本: MCP Server 1.3.0 | 更新日期: 2026-06-10  
> GridSim 版本: v3.0.0+

---

## 目录

1. [概述](#1-概述)
2. [快速开始](#2-快速开始)
3. [实例管理工具](#3-实例管理工具)
4. [数据接口工具](#4-数据接口工具)
5. [接口测试 (Proxy) 工具](#5-接口测试-proxy-工具)
6. [全局工具](#6-全局工具)
7. [自动变化策略详解](#7-自动变化策略详解)
8. [完整使用场景](#8-完整使用场景)
9. [故障排除](#9-故障排除)

---

## 1. 概述

MCP (Model Context Protocol) 服务器提供 GridSim 模拟器的程序化控制接口，使 AI 助手（如 Claude、GPT）可以直接操作模拟器。

### 工具分类

| 类别 | 工具数量 | 说明 |
|------|----------|------|
| 实例管理 | 9 个 | 创建/启停/删除/查询实例 |
| 数据接口 | 15 个 | 测点读写、策略配置、CSV 回放 |
| 接口测试 (Proxy) | 8 个 | HTTP 代理请求、集合/环境管理 |
| 全局工具 | 2 个 | 文件列表、协议查询 |
| **总计** | **34 个** | |

---

## 2. 快速开始

### 2.1 编译 MCP 程序

```bash
go build -o bin/mcp-server ./cmd/gridsim-mcp/
```

### 2.2 运行 MCP 服务器

```bash
# stdio 模式（推荐，用于 Claude Desktop 等）
./bin/mcp-server -simulator http://localhost:8989 -mode both
```

### 2.3 启动参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-simulator` | `http://localhost:8989` | GridSim 模拟器 HTTP 地址 |
| `-mode` | `both` | 运行模式：`instance`(实例管理) / `data`(数据接口) / `both`(全部) |

### 2.4 在 Claude Desktop 中配置

编辑 `claude_desktop_config.json`：

```json
{
  "mcpServers": {
    "gridsim": {
      "command": "/path/to/mcp-server",
      "args": ["-simulator", "http://localhost:8989", "-mode", "both"]
    }
  }
}
```

---

## 3. 实例管理工具

### 3.1 `list_instances`

列出所有已配置的模拟器实例。

**参数：** 无

**返回示例：**
```json
{
  "instances": [
    {
      "id": "a1b2c3d4e5f6",
      "name": "变电站A",
      "iec104_port": 2404,
      "xlsx_file": "samples/point.xlsx",
      "status": "running",
      "stats": { "uptime_seconds": 3600, "total_points": 7 }
    }
  ]
}
```

### 3.2 `get_instance`

获取单个实例的详细配置信息和运行状态。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `instance_id` | string | 是 | 实例 ID |

**返回示例：**
```json
{
  "id": "a1b2c3d4e5f6",
  "name": "变电站A",
  "iec104_port": 2404,
  "status": "running",
  "stats": {
    "uptime_seconds": 120,
    "total_points": 7,
    "client_connected": true,
    "interrogations": 5,
    "controls": 12,
    "spontaneous": 88
  }
}
```

### 3.3 `create_instance`

创建新的模拟器实例。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `config` | string | 是 | 实例配置 JSON 字符串 |

**config JSON 字段：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 实例名称 |
| `iec104_port` | int | 是 | IEC104 端口号 (1-65535) |
| `xlsx_file` | string | 条件 | 点表文件名（microgrid 除外） |
| `http_enabled` | bool | 否 | 是否启用实例级 HTTP |
| `http_port` | int | 条件 | HTTP 端口 |
| `protocol` | string | 否 | `iec104` / `modbus_tcp` / `microgrid` |

**使用示例：**
```python
create_instance(
    config='{"name": "变电站A", "iec104_port": 2404, "xlsx_file": "samples/point.xlsx"}'
)
```

### 3.4 `update_instance`

更新已有实例的配置。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `instance_id` | string | 是 | 实例 ID |
| `config` | string | 是 | 更新后的实例配置 JSON |

> 如正在运行，会先停止再更新。

**使用示例：**
```python
update_instance(
    instance_id="a1b2c3d4e5f6",
    config='{"name": "变电站A-修改", "iec104_port": 2405}'
)
```

### 3.5 `delete_instance`

删除实例（如运行中会先停止）。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `instance_id` | string | 是 | 实例 ID |

### 3.6 `start_instance`

启动指定实例。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `instance_id` | string | 是 | 实例 ID |

### 3.7 `stop_instance`

停止指定实例。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `instance_id` | string | 是 | 实例 ID |

### 3.8 `restart_instance`

重启指定实例。如未运行则直接启动。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `instance_id` | string | 是 | 实例 ID |

### 3.9 `get_server_status`

获取模拟器全局状态。

**参数：** 无

**返回示例：**
```json
{
  "version": "3.0.0",
  "mode": "serve",
  "configured": 5,
  "running": 3,
  "stopped": 2,
  "max": 1000
}
```

---

## 4. 数据接口工具

### 4.1 `list_points`

列出实例的所有测点及其当前值，按 AI 优先 + IOA 升序排列。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `instance_id` | string | 是 | 实例 ID |

**返回示例：**
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
      "updated_at": "2026-05-12T02:30:00.000Z"
    }
  ]
}
```

### 4.2 `read_point`

读取单个测点的当前值。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `instance_id` | string | 是 | 实例 ID |
| `ioa` | number | 是 | 信息体地址 |

### 4.3 `read_points`

批量读取多个测点的当前值。不传 `ioas` 则返回全部测点。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `instance_id` | string | 是 | 实例 ID |
| `ioas` | array[number] | 否 | 要读取的 IOA 列表，不传则返回全部 |

**使用示例：**
```python
read_points(instance_id="a1b2c3d4e5f6", ioas=[16385, 16386, 16387])
```

### 4.4 `write_point` ⭐

写入单个测点的值。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `instance_id` | string | 是 | 实例 ID |
| `ioa` | number | 是 | 信息体地址 |
| `value` | number | 否 | 浮点数值（AI 遥测使用），DI 也可用（非零=true） |
| `bool_value` | boolean | 否 | 布尔值（DI 遥信使用） |
| `int_value` | number | 否 | 整数值（PI 遥脉使用） |

> 根据测点类型选择对应字段。写入后触发 IEC104 变化上送 (COT=3)。

**使用示例：**
```python
# 设置 AI 遥测值
write_point(instance_id="a1b2c3d4e5f6", ioa=16385, value=235.5)

# 设置 DI 遥信值
write_point(instance_id="a1b2c3d4e5f6", ioa=5, bool_value=True)

# 设置 PI 遥脉值
write_point(instance_id="a1b2c3d4e5f6", ioa=10, int_value=42)
```

### 4.5 `write_points` ⭐ 核心

**批量写入多个测点的值** — 这是自动化测试的关键接口。一次调用写入多个 IOA，模拟真实设备同一时刻上报数据。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `instance_id` | string | 是 | 实例 ID |
| `points` | array | 是 | 要写入的测点列表 |

每个 point 元素：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `ioa` | number | 是 | 信息体地址 |
| `value` | number | 否 | 浮点数值 |
| `bool_value` | boolean | 否 | 布尔值 |
| `int_value` | number | 否 | 整数值 |

**使用示例：**
```python
write_points(
    instance_id="a1b2c3d4e5f6",
    points=[
        {"ioa": 16385, "value": 235.5},
        {"ioa": 16386, "value": 236.0},
        {"ioa": 16387, "bool_value": True}
    ]
)
```

**返回示例：**
```json
{
  "success": true,
  "results": [
    {"ioa": 16385, "success": true, "changed": true},
    {"ioa": 16386, "success": true, "changed": true},
    {"ioa": 16387, "success": true, "changed": true}
  ],
  "total": 3,
  "succeeded": 3,
  "failed": 0
}
```

### 4.6 `config_auto_change` ⭐

配置测点的自动变化策略。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `instance_id` | string | 是 | 实例 ID |
| `ioa` | number | 是 | 信息体地址 |
| `strategy` | string | 是 | 策略类型（见下表） |
| `enabled` | boolean | 是 | 是否启用 |
| `params` | string | 否 | 策略参数 JSON 字符串 |

**支持策略：**

| 策略名 | 说明 | params 示例 |
|--------|------|-------------|
| `increment` | 递增 | `{"start_value":0,"step":1,"period_ms":1000,"max_value":100}` |
| `random` | 随机 | `{"min_value":0,"max_value_r":100,"period_ms":1000,"decimal_places":2}` |
| `csv` | CSV回放 | `{"csv_file":"data.csv","time_format":"relative","time_unit":"ms","csv_column_map":"{\"1\":16385}"}` |
| `max` | 取大 | `{"para_a":"16385;16386"}` |
| `min` | 取小 | `{"para_a":"16385;16386"}` |
| `soc` | SOC计算 | `{"init_soc":50,"rated_cap":100,"power_ioa":16385,"integral_ms":1000}` |
| `energy` | 电量统计 | `{"init_energy":0,"stat_type":0,"energy_power_ioa":16385,"energy_period_ms":1000}` |
| `aofollow` | AO关联 | `{"follow_ao_ioa":10001}` |
| `apiupdate` | 接口更新 | `{"api_init_value":0}` |
| `manual` | 手动 | 无参数 |
| `custom` | 自定义公式 | `{"custom_ioas":"16385,16386","custom_formula":"{0}+{1}","period_ms":1000}` |

**使用示例：**
```python
# 递增策略
config_auto_change(
    instance_id="a1b2c3d4e5f6",
    ioa=16385,
    strategy="increment",
    enabled=True,
    params='{"start_value":0,"step":1,"period_ms":1000,"max_value":100}'
)

# 随机策略
config_auto_change(
    instance_id="a1b2c3d4e5f6",
    ioa=16386,
    strategy="random",
    enabled=True,
    params='{"min_value":200,"max_value_r":240,"period_ms":500,"decimal_places":1}'
)

# 手动策略（不自动计算，等待 API 置数）
config_auto_change(
    instance_id="a1b2c3d4e5f6",
    ioa=16385,
    strategy="manual",
    enabled=True
)
```

### 4.7 `batch_config_auto_change`

批量配置多个测点的同一策略。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `instance_id` | string | 是 | 实例 ID |
| `ioas` | array[number] | 是 | IOA 列表 |
| `strategy` | string | 是 | 策略类型 |
| `enabled` | boolean | 是 | 是否启用 |
| `params` | string | 否 | 策略参数 JSON |

**使用示例：**
```python
batch_config_auto_change(
    instance_id="a1b2c3d4e5f6",
    ioas=[16385, 16386, 16387],
    strategy="increment",
    enabled=True,
    params='{"start_value":0,"step":1,"period_ms":1000,"max_value":100}'
)
```

### 4.8 `get_auto_change`

查看测点的自动变化配置。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `instance_id` | string | 是 | 实例 ID |
| `ioa` | number | 是 | 信息体地址 |

### 4.9 `delete_auto_change`

删除测点的自动变化配置。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `instance_id` | string | 是 | 实例 ID |
| `ioa` | number | 是 | 信息体地址 |

### 4.10 `export_auto_changes`

导出实例所有自动变化配置为 CSV 表格。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `instance_id` | string | 是 | 实例 ID |

**返回：** CSV 内容字符串，包含策略代码和 A~G 参数列。

### 4.11 `import_auto_changes`

从 CSV 内容导入自动变化配置。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `instance_id` | string | 是 | 实例 ID |
| `csv_content` | string | 是 | CSV 内容 |

**使用示例：**
```python
import_auto_changes(
    instance_id="a1b2c3d4e5f6",
    csv_content="信息体地址,测点名称,自动变化模式,A,B,C,D\n16385,母线电压,1,0,1,1000,100"
)
```

### 4.12 `upload_csv`

上传 CSV 时间序列文件，用于 CSV 回放策略。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `instance_id` | string | 是 | 实例 ID |
| `csv_content` | string | 是 | CSV 文件内容 |

**使用示例：**
```python
upload_csv(
    instance_id="a1b2c3d4e5f6",
    csv_content="""time,母线电压,线路电流,有功功率
0,220.0,5.2,1144.0
1000,221.5,5.3,1173.9
2000,219.8,5.1,1120.9"""
)
```

### 4.13 `list_csv_files`

列出实例可用的 CSV 回放文件（共享目录 + 实例私有目录）。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `instance_id` | string | 是 | 实例 ID |

### 4.14 `config_csv_replay` ⭐ 核心

**配置 CSV 多测点同步回放** — 一键设置文件/时间/映射，自动为所有映射测点启用 CSV 回放策略。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `instance_id` | string | 是 | 实例 ID |
| `csv_file` | string | 是 | CSV 文件名（需提前上传） |
| `time_format` | string | 否 | `relative`(默认) / `absolute` |
| `time_unit` | string | 否 | `ms`(默认) / `s`，仅 relative 有效 |
| `csv_loop` | boolean | 否 | 是否循环播放，默认 true |
| `mappings` | array | 是 | 列到测点映射列表 |

每个 mapping 元素：

| 字段 | 类型 | 说明 |
|------|------|------|
| `column` | number | CSV 列序号（从 1 开始） |
| `ioa` | number | 测点 IOA |

> 最多 10 个映射。

**使用示例：**
```python
config_csv_replay(
    instance_id="a1b2c3d4e5f6",
    csv_file="replay_data.csv",
    time_format="relative",
    time_unit="ms",
    csv_loop=True,
    mappings=[
        {"column": 1, "ioa": 16385},
        {"column": 2, "ioa": 16386},
        {"column": 3, "ioa": 16387}
    ]
)
```

### 4.15 `update_qds`

更新测点的品质描述 QDS（传统模式 API）。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `ioa` | number | 是 | 信息体地址 |
| `invalid` | boolean | 否 | 无效标志 |
| `not_topical` | boolean | 否 | 非当前标志 |
| `substituted` | boolean | 否 | 替代标志 |
| `overflow` | boolean | 否 | 溢出标志 |
| `blocked` | boolean | 否 | 闭锁标志 |

**使用示例：**
```python
update_qds(ioa=16385, invalid=True, blocked=True)
```

---

## 5. 接口测试 (Proxy) 工具

### 5.1 `proxy_request`

发送 HTTP 代理请求，支持 GET/POST/PUT/DELETE/PATCH，可自定义 Headers 和 Body，变量会在发送前自动替换。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `method` | string | 是 | HTTP 方法: GET/POST/PUT/DELETE/PATCH |
| `url` | string | 是 | 目标 URL |
| `headers` | string | 否 | Headers JSON 字符串 |
| `body` | string | 否 | 请求体内容 |
| `timeout` | number | 否 | 超时秒数，默认 30 |

> URL 和 body 中的 `{{变量名}}` 会被当前激活的环境变量替换。

**使用示例：**
```python
# GET 请求
proxy_request(
    method="GET",
    url="https://api.example.com/data",
    headers='{"Authorization": "Bearer token123"}',
    timeout=30
)

# POST 请求
proxy_request(
    method="POST",
    url="{{base_url}}/api/realtime",
    headers='{"Content-Type": "application/json", "Authorization": "Bearer {{token}}"}',
    body='{"device_id": "PV-001"}'
)
```

**返回示例：**
```json
{
  "status": 200,
  "status_text": "OK",
  "headers": {"Content-Type": "application/json"},
  "body": "{\"data\": \"...\"}",
  "time_ms": 123,
  "size": 456
}
```

### 5.2 `get_collections`

获取接口测试中的所有请求集合和文件夹。

**参数：** 无

**返回示例：**
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
          "headers": {"Content-Type": "application/json"},
          "body": "",
          "pre_script": "",
          "test_script": ""
        }
      ]
    }
  ]
}
```

### 5.3 `save_collection`

保存/更新一个请求或文件夹。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `item` | string | 是 | 请求/文件夹的 JSON 字符串 |

**item JSON 字段：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | string | 唯一标识 |
| `name` | string | 名称 |
| `type` | string | `request` 或 `folder` |
| `method` | string | HTTP 方法（request 类型） |
| `url` | string | URL（request 类型） |
| `headers` | object | 请求头 |
| `body` | string | 请求体 |
| `pre_script` | string | 前置脚本 |
| `test_script` | string | 后置脚本 |
| `children` | array | 子项列表（folder 类型） |

**使用示例：**
```python
# 创建文件夹
save_collection(
    item='{"id":"req-001","name":"电力系统API","type":"folder","children":[]}'
)

# 创建请求
save_collection(
    item='{"id":"req-002","name":"获取实时数据","type":"request","method":"GET","url":"{{base_url}}/api/realtime","headers":{"Content-Type":"application/json"},"body":"","pre_script":"","test_script":""}'
)
```

### 5.4 `delete_collection`

删除一个请求或文件夹。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | string | 是 | 请求或文件夹的 ID |

### 5.5 `get_environments`

获取所有环境变量环境和当前活跃环境。

**参数：** 无

**返回示例：**
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

### 5.6 `save_environment`

保存/更新一个环境变量环境。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `env` | string | 是 | 环境的 JSON 字符串 |

**使用示例：**
```python
save_environment(
    env='{"id":"env-001","name":"测试环境","variables":{"base_url":"http://10.65.99.13:8989","token":"dev-token-abc"}}'
)
```

### 5.7 `activate_environment`

激活一个环境变量环境，使其成为当前使用的环境。后续请求中的 `{{变量名}}` 会自动替换。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | string | 是 | 环境的 ID |

### 5.8 `delete_environment`

删除一个环境变量环境。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | string | 是 | 环境的 ID |

### 5.9 `export_proxy_config`

导出全部接口测试配置（请求集合 + 环境变量）为 JSON 格式。

**参数：** 无

**返回示例：**
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

## 6. 全局工具

### 6.1 `list_files`

列出 config 目录下所有 `.xlsx` 点表文件。

**参数：** 无

**返回示例：**
```json
{
  "files": [
    {"name": "point.xlsx", "size": 12345, "modtime": "2026-05-12T10:30:00Z"}
  ]
}
```

### 6.2 `get_protocols`

查询模拟器支持的协议类型。

**参数：** 无

**返回示例：**
```json
{
  "protocols": ["iec104", "modbus_tcp", "microgrid"]
}
```

---

## 7. 自动变化策略详解

### 策略类型速查

| 代码 | 策略名 | 说明 | 适用测点 |
|------|--------|------|----------|
| 1 | `increment` | 递增 | AI/PI |
| 2 | `random` | 随机 | AI |
| 3 | `csv` | CSV 回放 | AI/DI/PI |
| 4 | `max` | 取大 | AI |
| 5 | `min` | 取小 | AI |
| 6 | `soc` | SOC 计算 | AI |
| 7 | `energy` | 电量统计 | AI |
| 8 | `aofollow` | AO 关联 | AI |
| 9 | `apiupdate` | 接口更新 | AI/DI/PI |
| 10 | `manual` | 手动 | AI/DI/PI |
| 11 | `custom` | 自定义公式 | AI |

> AO/DO 类型不支持自动变化。

### 策略参数详解

#### increment（递增）

从 `start_value` 开始，每 `period_ms` 毫秒增加 `step`，达到 `max_value` 后回到 `start_value`。

```json
{
  "start_value": 0,
  "step": 1,
  "period_ms": 1000,
  "max_value": 100
}
```

#### random（随机）

在 `[min_value, max_value_r]` 范围内每 `period_ms` 毫秒产生随机值，保留 `decimal_places` 位小数。

```json
{
  "min_value": 200,
  "max_value_r": 240,
  "period_ms": 500,
  "decimal_places": 1
}
```

#### csv（CSV 回放）

按 CSV 文件定义的时间序列播放。`csv_column_map` 指定列到测点的映射。

```json
{
  "csv_file": "replay_data.csv",
  "time_format": "relative",
  "time_unit": "ms",
  "csv_column_map": "{\"1\":16385}",
  "csv_loop": true
}
```

> 推荐使用 `config_csv_replay` 工具替代直接配置，更方便。

#### max/min（取大/取小）

从多个 IOA 中取最大/最小值。

```json
{
  "para_a": "16385;16386;16387",
  "para_b": "16385"
}
```

#### soc（SOC 计算）

基于功率积分计算电池荷电状态。

```json
{
  "init_soc": 50,
  "rated_cap": 100,
  "power_ioa": 16385,
  "integral_ms": 1000
}
```

#### energy（电量统计）

基于功率积分计算累计电量。

```json
{
  "init_energy": 0,
  "stat_type": 0,
  "energy_power_ioa": 16385,
  "energy_period_ms": 1000
}
```

- `stat_type`: 0=充电电量，1=放电电量

#### aofollow（AO 关联）

跟随指定 AO 点的遥控值变化。

```json
{
  "follow_ao_ioa": 10001
}
```

#### apiupdate（接口更新）

仅允许 HTTP API 写入，引擎不做自动计算。设置初始值。

```json
{
  "api_init_value": 0
}
```

#### manual（手动）

不自动计算，需通过 API 置数。适用于外部系统联调场景。无参数。

#### custom（自定义公式）

支持四则运算公式，2~50 个关联测点。

```json
{
  "custom_ioas": "16385,16386,16387",
  "custom_formula": "{0}+{1}*{2}",
  "period_ms": 1000
}
```

- `custom_ioas`: 关联测点 IOA 列表，逗号分隔
- `custom_formula`: 公式，用 `{0}`, `{1}`, ... 引用关联测点的值
- `period_ms`: 计算周期（≥100ms）
- 至少 2 个关联测点，最多 50 个

---

## 8. 完整使用场景

### 场景 1: 从零创建并启动一个仿真实例

```python
# 1. 查看可用文件
list_files()

# 2. 创建实例
result = create_instance(
    config='{"name":"变电站A","iec104_port":2404,"xlsx_file":"samples/point.xlsx"}'
)
# 记下返回的 instance_id

# 3. 启动实例
start_instance(instance_id="a1b2c3d4e5f6")

# 4. 查看运行状态
get_instance(instance_id="a1b2c3d4e5f6")

# 5. 查看所有测点
list_points(instance_id="a1b2c3d4e5f6")
```

### 场景 2: 模拟数据变化并验证

```python
# 1. 配置自动变化策略
config_auto_change(
    instance_id="a1b2c3d4e5f6",
    ioa=16385,
    strategy="increment",
    enabled=True,
    params='{"start_value":0,"step":1,"period_ms":1000,"max_value":100}'
)

# 2. 批量写入测试数据
write_points(
    instance_id="a1b2c3d4e5f6",
    points=[
        {"ioa": 16385, "value": 235.5},
        {"ioa": 16386, "value": 236.0},
        {"ioa": 5, "bool_value": True}
    ]
)

# 3. 读取验证
read_point(instance_id="a1b2c3d4e5f6", ioa=16385)
```

### 场景 3: CSV 多测点同步回放

```python
# 1. 上传 CSV 文件
upload_csv(
    instance_id="a1b2c3d4e5f6",
    csv_content="""time,voltage,current,power
0,220.0,5.2,1144.0
1000,221.5,5.3,1173.9
2000,219.8,5.1,1120.9"""
)

# 2. 一键配置回放
config_csv_replay(
    instance_id="a1b2c3d4e5f6",
    csv_file="replay_data.csv",
    time_format="relative",
    time_unit="ms",
    mappings=[
        {"column": 1, "ioa": 16385},
        {"column": 2, "ioa": 16386},
        {"column": 3, "ioa": 16387}
    ]
)

# 3. 观察数据变化
list_points(instance_id="a1b2c3d4e5f6")
```

### 场景 4: 接口测试 — 发送 HTTP 请求

```python
# 1. 查看环境变量
get_environments()

# 2. 激活环境
activate_environment(id="env-xxx")

# 3. 发送请求
proxy_request(
    method="GET",
    url="{{base_url}}/api/v1/status",
    headers='{"Authorization": "Bearer {{token}}"}'
)
```

### 场景 5: 接口测试 — 创建和管理 API 集合

```python
# 1. 创建文件夹
save_collection(
    item='{"id":"folder-001","name":"GridSim API 测试","type":"folder","children":[]}'
)

# 2. 在文件夹下创建接口
save_collection(
    item='{"id":"req-001","name":"获取全局状态","type":"request","method":"GET","url":"{{base_url}}/api/v1/status","headers":{},"body":"","pre_script":"","test_script":"pm.response.to.have.status(200)"}'
)

# 3. 查看所有集合
get_collections()

# 4. 执行请求
proxy_request(method="GET", url="{{base_url}}/api/v1/status")

# 5. 清理
delete_collection(id="req-001")
```

---

## 9. 故障排除

### 连接失败

```
错误: dial tcp connection refused
```

**排查步骤：**
1. 确认 GridSim 模拟器已启动：`./bin/gridsim serve --http :8989`
2. 检查 `-simulator` 参数地址是否正确
3. 测试连通性：`curl http://localhost:8989/api/v1/status`

### 测点写入失败

```
错误: IOA xxx not found / point not found
```

**排查步骤：**
1. 确认实例已启动（`get_instance` 查看状态为 `running`）
2. 用 `list_points` 查看实际 IOA 列表
3. 检查点表 xlsx 是否正确加载

### 自动变化配置失败

```
错误: AO/DO does not support auto-change
```

AO/DO 类型测点不支持自动变化策略。只能为 AI、DI、PI 类型配置。

```
错误: period_ms must be >= 100
```

策略计算周期最小 100ms。

```
错误: custom formula requires at least 2 associated IOAs
```

自定义公式策略至少需要关联 2 个测点。

### MCP 工具无响应

1. 检查 GridSim 模拟器日志
2. 确认点表文件已正确加载
3. 验证实例状态为 `running`（使用 `get_instance` 或 `get_server_status`）
4. 检查 MCP 服务器是否正常运行

### 实例启动端口冲突

```
错误: port 2404 already in use by instance xxx
```

使用 `list_instances` 查看已使用的端口，更换端口或先停止占用端口的实例。

---

## 版本信息

| 组件 | 版本 |
|------|------|
| GridSim | 3.0.0+ |
| MCP Server | 1.3.0 |
| 工具总数 | 34 (实例管理 9 + 数据接口 15 + 接口测试 8 + 全局 2) |
