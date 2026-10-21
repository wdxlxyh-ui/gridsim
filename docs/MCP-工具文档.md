# GridSim MCP 使用指南

> 适用版本：GridSim `3.2.1-dev+9.64e7f61`  
> Git 提交：`64e7f61`  
> 核对日期：2026-08-14  
> MCP 工具数量：37

本文说明 GridSim MCP 服务的安装、配置、工具参数、典型调用和当前限制。适用于通过 Kiro、Claude Desktop 或其他支持 MCP `stdio` 传输的客户端管理 GridSim 实例、读写测点、配置自动变化策略和调用接口测试工具。

本文以当前源码和已部署二进制为准。旧文档中的 `./cmd/mcp-server/`、28 个工具、`upload_file`、`export_points_csv` 以及旧版 Proxy 工具名均已过期。

## 1. 工作方式

GridSim MCP 不是独立的网络服务，不监听额外端口。MCP 客户端启动 `gridsim-mcp` 子进程，通过标准输入和标准输出交换 MCP 消息；`gridsim-mcp` 再通过 HTTP 调用 GridSim REST API。

```text
Kiro / Claude Desktop / MCP Client
                │
                │ stdio
                ▼
          gridsim-mcp
                │
                │ HTTP REST API
                ▼
      GridSim（默认端口 8989）
```

当前实现特征：

- MCP 传输：仅支持 `stdio`。
- 不支持：SSE、Streamable HTTP、独立 MCP 监听端口。
- GridSim API 默认地址：`http://localhost:8989`。
- MCP 客户端不附加 Token、Cookie 或 API Key。
- 工具结果统一以 MCP 文本内容返回；JSON 会格式化后作为文本返回。
- REST 请求失败时，工具通常返回以 `Error:` 开头的文本，而不是 MCP 协议级错误。

## 2. 版本与文件位置

### 2.1 发布包中的文件

Linux 发布包：

```text
bin/gridsim
bin/gridsim-mcp
bin/VERSION
```

Windows 发布包：

```text
bin/gridsim.exe
bin/gridsim-mcp.exe
bin/VERSION
```

当前编译服务器的部署路径：

```text
/home/envuser/IEC/gridsim/bin/gridsim-mcp
```

当前版本的构建产物：

```text
/root/IEC-SIM/iec104-sim-master/dist/gridsim-v3.2.1-dev-9.64e7f61-linux-amd64.tar.gz
/root/IEC-SIM/iec104-sim-master/dist/gridsim-v3.2.1-dev-9.64e7f61-linux-arm64.tar.gz
/root/IEC-SIM/iec104-sim-master/dist/gridsim-v3.2.1-dev-9.64e7f61-windows-amd64.zip
```

### 2.2 从源码构建

```bash
go build -o bin/gridsim-mcp ./cmd/gridsim-mcp/
```

查看启动参数：

```bash
./bin/gridsim-mcp -h
```

## 3. 启动参数与运行模式

```bash
./bin/gridsim-mcp \
  -simulator http://localhost:8989 \
  -mode both
```

| 参数 | 默认值 | 说明 |
|---|---|---|
| `-simulator` | `http://localhost:8989` | GridSim REST API 基础地址，不要附加 `/api/v1` |
| `-mode` | `both` | 工具模式：`instance`、`data`、`proxy` 或 `both` |

### 3.1 模式与工具数量

| 模式 | 工具数 | 内容 |
|---|---:|---|
| `instance` | 11 | 实例生命周期 9 个、点表文件查询、协议查询 |
| `data` | 17 | 测点读写、自动变化、CSV 回放、QDS、OpenAPI、统一状态 |
| `proxy` | 9 | HTTP 代理请求、请求集合和环境变量管理 |
| `both` | 37 | 合并以上全部工具，推荐使用 |

说明：当前 `-h` 输出中的模式说明遗漏了 `proxy`，但源码和已构建二进制支持 `-mode proxy`。

### 3.2 GridSim 与 MCP 位于同一台 Linux 服务器

```bash
/home/envuser/IEC/gridsim/bin/gridsim-mcp \
  -simulator http://127.0.0.1:8989 \
  -mode both
```

### 3.3 MCP 在 Windows、GridSim 在远程服务器

先从 Windows 发布包中取得 `gridsim-mcp.exe`，再指定远程 GridSim 地址：

```powershell
D:\GridSim\bin\gridsim-mcp.exe `
  -simulator http://10.230.169.162:8989 `
  -mode both
```

该方式要求 Windows 主机可以访问远程服务器的 8989 端口。

## 4. Kiro 配置

Kiro 工作区配置文件：

```text
.kiro/settings/mcp.json
```

用户级配置文件：

```text
~/.kiro/settings/mcp.json
```

### 4.1 Windows 本地 MCP 连接远程 GridSim

```json
{
  "mcpServers": {
    "gridsim": {
      "command": "D:\\GridSim\\bin\\gridsim-mcp.exe",
      "args": [
        "-simulator",
        "http://10.230.169.162:8989",
        "-mode",
        "both"
      ],
      "disabled": false
    }
  }
}
```

### 4.2 Linux 本机配置

```json
{
  "mcpServers": {
    "gridsim": {
      "command": "/home/envuser/IEC/gridsim/bin/gridsim-mcp",
      "args": [
        "-simulator",
        "http://127.0.0.1:8989",
        "-mode",
        "both"
      ],
      "disabled": false
    }
  }
}
```

执行 MCP 进程的系统账号必须具有 `gridsim-mcp` 的执行权限。

### 4.3 通过 SSH 启动远程 MCP

当本地不保存 MCP 二进制时，可以通过 SSH 将 `stdio` 转发到远程服务器。此方式必须配置免交互 SSH 密钥，不能在 MCP 启动过程中等待密码输入。

```json
{
  "mcpServers": {
    "gridsim-remote": {
      "command": "C:\\Windows\\System32\\OpenSSH\\ssh.exe",
      "args": [
        "-T",
        "root@10.230.169.162",
        "/home/envuser/IEC/gridsim/bin/gridsim-mcp",
        "-simulator",
        "http://127.0.0.1:8989",
        "-mode",
        "both"
      ],
      "disabled": false
    }
  }
}
```

配置保存后，在 Kiro 的 MCP Server 视图中重新连接服务。

## 5. Claude Desktop 配置

Windows 配置文件通常位于：

```text
%APPDATA%\Claude\claude_desktop_config.json
```

配置示例：

```json
{
  "mcpServers": {
    "gridsim": {
      "command": "D:\\GridSim\\bin\\gridsim-mcp.exe",
      "args": [
        "-simulator",
        "http://10.230.169.162:8989",
        "-mode",
        "both"
      ]
    }
  }
}
```

配置修改后需要完全退出并重新启动 Claude Desktop。

## 6. 使用前检查

先确认 GridSim API 正常：

```bash
curl http://10.230.169.162:8989/api/v1/status
```

预期返回类似：

```json
{
  "configured": 6,
  "git_commit": "64e7f61",
  "mode": "serve",
  "running": 3,
  "stopped": 3,
  "version": "3.2.1-dev+9.64e7f61"
}
```

MCP 连接完成后，建议按以下顺序检查：

1. 调用 `get_state` 获取统一状态。
2. 调用 `list_instances` 获取实例 ID。
3. 调用 `get_instance` 确认目标实例状态。
4. 调用 `list_points` 获取实际 IOA 和点类型。
5. 再执行写值、启停或配置修改。

## 7. 工具总览

### 7.1 实例管理模式：11 个工具

| 工具 | 参数 | 用途 |
|---|---|---|
| `list_instances` | 无 | 列出所有实例及运行状态 |
| `get_instance` | `instance_id` | 获取单个实例配置和统计信息 |
| `create_instance` | `config` | 使用 JSON 字符串创建实例 |
| `update_instance` | `instance_id`、`config` | 更新实例配置 |
| `delete_instance` | `instance_id` | 删除实例 |
| `start_instance` | `instance_id` | 启动实例 |
| `stop_instance` | `instance_id` | 停止实例 |
| `restart_instance` | `instance_id` | 重启实例 |
| `get_server_status` | 无 | 获取 GridSim 全局状态 |
| `list_files` | 无 | 列出配置目录中的 `.xlsx` 点表 |
| `get_protocols` | 无 | 查询支持的协议类型 |

### 7.2 数据模式：17 个工具

| 工具 | 参数 | 用途 |
|---|---|---|
| `list_points` | `instance_id` | 列出实例全部测点和当前值 |
| `read_point` | `instance_id`、`ioa` | 读取一个测点 |
| `read_points` | `instance_id`、`ioas?` | 读取多个测点；不传 `ioas` 时返回全部 |
| `write_point` | `instance_id`、`ioa`、值字段 | 写入一个 AI、DI 或 PI 点 |
| `write_points` | `instance_id`、`points` | 批量写入多个 AI、DI 或 PI 点 |
| `config_auto_change` | `instance_id`、`ioa`、`strategy`、`enabled`、`params?` | 配置一个测点的自动变化策略 |
| `batch_config_auto_change` | `instance_id`、`ioas`、`strategy`、`enabled`、`params?` | 为多个测点配置相同策略 |
| `get_auto_change` | `instance_id`、`ioa` | 查询自动变化配置 |
| `delete_auto_change` | `instance_id`、`ioa` | 删除自动变化配置 |
| `export_auto_changes` | `instance_id` | 导出自动变化配置 CSV |
| `import_auto_changes` | `instance_id`、`csv_content` | 从 CSV 文本导入自动变化配置 |
| `upload_csv` | `instance_id`、`csv_content` | 已注册，但当前版本存在上传格式兼容问题 |
| `list_csv_files` | `instance_id` | 列出共享和实例私有 CSV 文件 |
| `config_csv_replay` | `instance_id`、`csv_file`、时间参数、`mappings` | 配置多测点同步回放 |
| `update_qds` | `ioa`、QDS 标志 | 通过传统接口更新点品质 |
| `get_openapi_spec` | 无 | 获取 GridSim OpenAPI 3.0 文档 |
| `get_state` | 无 | 一次获取全部实例状态概要 |

### 7.3 Proxy 模式：9 个工具

| 工具 | 参数 | 用途 |
|---|---|---|
| `proxy_request` | `method`、`url`、`headers?`、`body?`、`timeout?` | 通过 GridSim 发送 HTTP 请求 |
| `get_collections` | 无 | 获取请求集合和文件夹 |
| `save_collection` | `item` | 保存请求或文件夹 JSON |
| `delete_collection` | `id` | 删除请求或文件夹 |
| `get_environments` | 无 | 获取环境变量列表和当前环境 |
| `save_environment` | `env` | 保存环境变量 JSON |
| `delete_environment` | `id` | 删除环境 |
| `activate_environment` | `id` | 激活环境 |
| `export_proxy_config` | 无 | 导出请求集合和环境配置 |

## 8. 实例管理示例

MCP 客户端通常会以表单方式展示工具参数。以下示例仅展示参数 JSON，不代表需要手工发送 JSON-RPC。

### 8.1 查询实例

工具：`list_instances`

```json
{}
```

工具：`get_instance`

```json
{
  "instance_id": "f2f210dd7027"
}
```

### 8.2 创建 IEC 104 实例

工具：`create_instance`

`config` 参数本身是 JSON 字符串：

```json
{
  "config": "{\"name\":\"IEC104-Test\",\"iec104_port\":2404,\"xlsx_file\":\"point.xlsx\",\"protocol\":\"iec104\"}"
}
```

### 8.3 创建 Modbus TCP 实例

```json
{
  "config": "{\"name\":\"Modbus-Test\",\"iec104_port\":5778,\"xlsx_file\":\"ModbusTC.xlsx\",\"protocol\":\"modbus_tcp\",\"modbus_config\":{\"port\":5778,\"byte_order\":\"CDAB\",\"slave_id\":1}}"
}
```

说明：

- `iec104_port` 是实例模型中的通用端口字段；Modbus TCP 实例仍需填写。
- `modbus_config.port` 应与实例监听端口一致。
- `byte_order` 支持 `ABCD`、`CDAB`、`BADC`、`DCBA`。
- 创建前先调用 `list_files` 确认点表文件名。

### 8.4 启停实例

工具：`start_instance`

```json
{
  "instance_id": "f2f210dd7027"
}
```

`stop_instance` 和 `restart_instance` 使用相同参数结构。

实例启停会改变运行状态。自动化流程执行前应先调用 `get_instance`，避免重复启动或误停其他测试实例。

## 9. 测点读写示例

### 9.1 获取全部点

工具：`list_points`

```json
{
  "instance_id": "f2f210dd7027"
}
```

返回数据包含：

- `ioa`：GridSim 内部信息体地址。
- `name`：点名。
- `point_type`：`AI`、`DI`、`PI`、`AO` 或 `DO`。
- `value`：浮点值。
- `bool_value`：布尔值。
- `int_value`：整数值。
- `updated_at`：最后更新时间。
- Modbus 点还可能包含 `function_code` 和 `register_address`。

### 9.2 批量读取指定点

工具：`read_points`

```json
{
  "instance_id": "f2f210dd7027",
  "ioas": [1, 3, 7]
}
```

### 9.3 写入单点

AI：

```json
{
  "instance_id": "inst-001",
  "ioa": 16385,
  "value": 235.5
}
```

DI：

```json
{
  "instance_id": "inst-001",
  "ioa": 1,
  "bool_value": true
}
```

PI：

```json
{
  "instance_id": "inst-001",
  "ioa": 2,
  "int_value": 42
}
```

`write_point` 是模拟器置数接口，仅允许 AI、DI、PI。AO 和 DO 代表客户端控制输入，不能通过该工具置数，应由对应协议客户端发送控制命令。

### 9.4 批量写入

工具：`write_points`

```json
{
  "instance_id": "inst-001",
  "points": [
    {"ioa": 16385, "value": 235.5},
    {"ioa": 16386, "value": 236.0},
    {"ioa": 1, "bool_value": true},
    {"ioa": 2, "int_value": 42}
  ]
}
```

同一个点只填写与其类型对应的值字段。

当前限制：`write_points` 的 MCP 参数转换会把 IOA 0 判断为无效。IOA 0 应使用 `write_point` 单独写入，或等待后续版本修复批量校验。

## 10. 自动变化策略

### 10.1 支持的策略

| 策略 | 用途 | 常用参数 |
|---|---|---|
| `increment` | 周期递增 | `start_value`、`step`、`period_ms`、`max_value` |
| `random` | 范围内随机变化 | `min_value`、`max_value_r`、`period_ms`、`decimal_places` |
| `csv` | CSV 时间序列回放 | `csv_file`、`time_format`、`time_unit`、映射配置 |
| `max` | 取多个点的最大值 | `para_a` |
| `min` | 取多个点的最小值 | `para_a` |
| `soc` | SOC 积分计算 | `init_soc`、`rated_cap`、`power_ioa`、`integral_ms` |
| `energy` | 电量积分 | `init_energy`、`stat_type`、`energy_power_ioa` |
| `aofollow` | 跟随 AO 控制值 | `follow_ao_ioa` |
| `apiupdate` | 仅通过接口更新 | `api_init_value` |
| `manual` | 保持手动值 | 无 |
| `custom` | 自定义公式计算 | `custom_ioas`、`custom_formula`、`period_ms` |

AO 和 DO 不支持自动变化策略。

### 10.2 配置单点递增

工具：`config_auto_change`

`params` 是 JSON 字符串：

```json
{
  "instance_id": "inst-001",
  "ioa": 16385,
  "strategy": "increment",
  "enabled": true,
  "params": "{\"start_value\":0,\"step\":1,\"period_ms\":1000,\"max_value\":100}"
}
```

### 10.3 批量配置随机变化

```json
{
  "instance_id": "inst-001",
  "ioas": [16385, 16386, 16387],
  "strategy": "random",
  "enabled": true,
  "params": "{\"min_value\":200,\"max_value_r\":240,\"period_ms\":1000,\"decimal_places\":1}"
}
```

建议先调用 `get_auto_change` 保存原配置，再执行修改；测试结束后调用 `delete_auto_change` 或恢复原配置。

## 11. CSV 回放

推荐流程：

1. 调用 `list_csv_files` 确认文件是否存在。
2. 调用 `config_csv_replay` 配置时间格式和列映射。
3. 调用 `list_points` 观察回放值。
4. 测试结束后删除或禁用对应自动变化配置。

工具：`config_csv_replay`

```json
{
  "instance_id": "inst-001",
  "csv_file": "replay_data.csv",
  "time_format": "relative",
  "time_unit": "ms",
  "csv_loop": true,
  "mappings": [
    {"column": 1, "ioa": 16385},
    {"column": 2, "ioa": 16386},
    {"column": 3, "ioa": 16387}
  ]
}
```

参数说明：

| 参数 | 必填 | 说明 |
|---|---|---|
| `csv_file` | 是 | 已存在的 CSV 文件名 |
| `time_format` | 否 | `relative` 或 `absolute`，默认 `relative` |
| `time_unit` | 否 | `ms` 或 `s`，默认 `ms` |
| `csv_loop` | 否 | 是否循环，默认 `true` |
| `mappings` | 是 | CSV 数据列到 IOA 的映射；列序号从 1 开始 |

当前版本的 `upload_csv` 工具存在接口格式不一致：MCP 客户端发送 `text/csv`，后端上传接口要求 `multipart/form-data`。该工具会返回类似 `no file provided` 的错误。当前应通过 Web 页面或 REST multipart 接口上传文件，再使用 `list_csv_files` 和 `config_csv_replay`。

REST 上传示例：

```bash
curl -X POST \
  -F "file=@replay_data.csv" \
  http://10.230.169.162:8989/api/v1/instances/inst-001/upload-csv
```

## 12. QDS 品质设置

工具：`update_qds`

```json
{
  "ioa": 16385,
  "invalid": true,
  "not_topical": false,
  "substituted": false,
  "overflow": false,
  "blocked": true
}
```

可选标志：

- `invalid`：无效。
- `not_topical`：非当前。
- `substituted`：替代。
- `overflow`：溢出。
- `blocked`：闭锁。

该工具调用传统接口 `/api/points/{ioa}/qds`，参数中没有 `instance_id`。多实例环境使用前应确认传统接口对应的实例上下文；无法确认时，优先使用明确包含实例 ID 的 REST 接口。

## 13. OpenAPI 与统一状态

### 13.1 获取 OpenAPI 文档

工具：`get_openapi_spec`

```json
{}
```

等价 REST 地址：

```text
http://10.230.169.162:8989/openapi.json
```

### 13.2 获取统一状态

工具：`get_state`

```json
{}
```

该工具适合会话开始时调用，一次返回实例状态、协议、端口、点数、客户端连接和运行时间等概要，减少多次查询。

## 14. Proxy 接口测试

Proxy 工具由 GridSim 服务端发出目标 HTTP 请求。目标地址必须是 GridSim 服务器能够访问的地址，而不是 MCP 客户端本机能够访问的地址。

### 14.1 直接发送请求

工具：`proxy_request`

```json
{
  "method": "GET",
  "url": "http://127.0.0.1:8989/api/v1/status",
  "headers": "{\"Accept\":\"application/json\"}",
  "timeout": 30
}
```

POST 示例：

```json
{
  "method": "POST",
  "url": "{{base_url}}/api/example",
  "headers": "{\"Content-Type\":\"application/json\",\"Authorization\":\"Bearer {{token}}\"}",
  "body": "{\"device_id\":\"PV-001\"}",
  "timeout": 30
}
```

### 14.2 保存并激活环境

工具：`save_environment`

```json
{
  "env": "{\"id\":\"env-test\",\"name\":\"测试环境\",\"variables\":{\"base_url\":\"http://127.0.0.1:8989\",\"token\":\"example-token\"}}"
}
```

工具：`activate_environment`

```json
{
  "id": "env-test"
}
```

激活后，Proxy 请求中的 `{{base_url}}`、`{{token}}` 等变量由 GridSim Proxy 模块替换。

### 14.3 保存请求集合

工具：`save_collection`

```json
{
  "item": "{\"id\":\"req-status\",\"name\":\"获取 GridSim 状态\",\"type\":\"request\",\"method\":\"GET\",\"url\":\"{{base_url}}/api/v1/status\",\"headers\":{},\"body\":\"\",\"pre_script\":\"\",\"test_script\":\"\"}"
}
```

`item` 和 `env` 都是 JSON 字符串，不是直接嵌套对象。

## 15. 返回值与错误处理

### 15.1 正常返回

REST API 返回的 JSON 会作为格式化文本放在 MCP `content` 中。MCP 客户端通常会直接展示为 JSON 文本。

### 15.2 错误返回

常见格式：

```text
Error: HTTP 404: [NOT_FOUND] instance not found
```

或：

```text
Error: GET /api/v1/status: dial tcp 10.230.169.162:8989: connect: connection refused
```

当前工具处理器不会把这类错误标记为 MCP 协议级 `isError=true`。自动化 Agent 应检查返回文本是否以 `Error:` 开头，不能只依据工具调用本身是否完成。

当 REST API 返回结构化错误时，MCP 会尽量保留：

- 错误代码 `code`。
- 错误说明 `message`。
- 修复提示 `hint`。
- 候选项 `candidates`。

## 16. 安全建议

- `start_instance`、`stop_instance`、`restart_instance`、`delete_instance` 和配置修改会影响正在运行的测试。
- `proxy_request` 可以由 GridSim 服务器向其他 HTTP 地址发起请求，只应在可信网络和受控账号下启用。
- 当前 MCP REST 客户端不支持认证头注入。需要认证的 GridSim 部署无法直接使用实例和数据工具。
- 不要在 MCP 配置文件中保存明文生产 Token。Proxy 环境变量同样应按敏感配置管理。
- 建议默认使用 `instance` 或 `data` 最小权限模式；确实需要接口测试时再启用 `proxy` 或 `both`。
- 对删除、停止、批量写值和批量策略配置操作，应先查询当前状态并保留原配置。

## 17. 当前版本限制

| 项目 | 当前行为 | 建议 |
|---|---|---|
| MCP 传输 | 仅 `stdio` | 由 MCP 客户端直接启动进程 |
| SSE / Streamable HTTP | 未实现 | 不要配置 MCP URL |
| MCP 自报版本 | 代码中固定为 `1.0.0` | 以 `bin/VERSION` 和 GridSim Git 提交识别发布版本 |
| `-h` 模式说明 | 未列出 `proxy` | 实际可使用 `-mode proxy` |
| REST 认证 | 不支持 Token、Cookie、API Key | 仅连接无需认证的可信 GridSim API |
| REST 全局超时 | 未设置 | 避免连接不可达地址；必要时由进程管理器控制超时 |
| `upload_csv` | 与后端 multipart 要求不匹配 | 使用 Web 页面或 REST multipart 上传 |
| `write_points` + IOA 0 | IOA 0 被判断为无效 | 使用 `write_point` 单独写入 |
| `update_qds` | 无 `instance_id` | 多实例环境谨慎使用 |
| `upload_file` | 客户端代码存在，但未注册为 MCP 工具 | 使用 Web 页面或 REST API 上传点表 |
| `export_points_csv` | 客户端代码存在，但未注册为 MCP 工具 | 使用 REST API 或 Web 页面导出 |

## 18. 故障排查

### 18.1 MCP 服务未出现

1. 检查 `command` 是否为 MCP 客户端所在机器上的绝对路径。
2. 在终端运行 `gridsim-mcp -h`，确认二进制可执行。
3. 检查操作系统架构是否匹配：Windows amd64、Linux amd64 或 Linux arm64。
4. 检查 Kiro 或 Claude Desktop 日志中的进程启动错误。
5. 修改配置后重新连接 MCP 服务或重启客户端。

### 18.2 工具存在但全部调用失败

检查 GridSim API：

```bash
curl http://10.230.169.162:8989/api/v1/status
```

检查 `-simulator`：

- 不要附加 `/api/v1`。
- 远程连接不要写 `localhost`，除非 MCP 进程也在 GridSim 服务器上。
- 确认防火墙允许访问 8989 端口。

### 18.3 实例启动失败

1. 调用 `get_instance` 查看配置。
2. 调用 `list_files` 确认点表存在。
3. 检查协议端口是否已被占用。
4. 检查点表功能码、寄存器地址和数据类型。
5. 读取工具返回文本中的 `message`、`hint` 和 `candidates`。

### 18.4 测点写入没有变化

1. 用 `list_points` 确认 IOA 和 `point_type`。
2. AI 使用 `value`，DI 使用 `bool_value`，PI 使用 `int_value`。
3. AO、DO 不允许通过置数接口写入。
4. 检查测点是否启用了禁止 API 写入的自动变化策略。
5. 再次调用 `read_point` 核对结果。

### 18.5 工具数量不是 37

检查 `-mode`：

```text
instance：11
数据模式：17
proxy：9
both：37
```

如果配置为 `both` 仍不是 37，检查是否使用了旧版 `mcp-server` 或旧安装包。

## 19. 推荐调用顺序

### 19.1 只读检查

```text
get_state
→ list_instances
→ get_instance
→ list_points
→ read_points
```

### 19.2 创建并启动实例

```text
list_files
→ get_protocols
→ create_instance
→ start_instance
→ get_instance
→ list_points
```

### 19.3 自动化数据注入

```text
get_instance
→ list_points
→ get_auto_change
→ write_points / config_auto_change
→ read_points
→ 恢复原配置
```

### 19.4 CSV 回放

```text
通过 Web 或 REST multipart 上传 CSV
→ list_csv_files
→ config_csv_replay
→ list_points
→ 删除或恢复自动变化配置
```

## 20. 维护说明

MCP 工具注册的事实来源：

```text
internal/mcp/server.go
```

MCP 到 REST API 的映射来源：

```text
internal/mcp/client.go
```

命令行入口：

```text
cmd/gridsim-mcp/main.go
```

构建与发布文件名：

```text
build.sh
docs/PACKAGING.md
```

当工具数量、参数或 REST 路由变化时，应优先更新本文件，避免继续引用根目录 `MCP.md`、旧版 `manuals/mcp-manual.md` 或知识库中的历史工具清单。
