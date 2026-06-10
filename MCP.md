# IEC 104 模拟器 MCP 使用指南

> 版本: 1.2 | 更新日期: 2026-06-04

## 概述

MCP (Model Context Protocol) 服务器提供 IEC 104 模拟器的程序化控制接口，支持：
- 实例管理（创建、启动、停止、删除）
- 测点数据读写（批量写入、策略配置）
- 文件上传（点表、CSV 回放文件）

## 快速开始

### 1. 编译 MCP 程序

```bash
# 编译 MCP 服务器
go build -o bin/mcp-server ./cmd/mcp-server/
```

### 2. 运行 MCP 服务器

```bash
# 方式一：使用 stdio 模式（推荐，用于 Claude Desktop 等）
./bin/mcp-server -simulator http://localhost:8989 -mode both

# 方式二：查看帮助
./bin/mcp-server -h
```

参数说明：
| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-simulator` | http://localhost:8989 | IEC104 模拟器 HTTP 地址 |
| `-mode` | both | 运行模式：instance(实例管理) / data(数据接口) / both(全部) |

## 可用工具

### 实例管理工具

| 工具名称 | 说明 |
|----------|------|
| `list_instances` | 列出所有已配置的模拟器实例 |
| `get_instance` | 获取单个实例的详细配置信息 |
| `create_instance` | 创建新的模拟器实例 |
| `update_instance` | 更新已有实例的配置 |
| `delete_instance` | 删除实例 |
| `start_instance` | 启动指定实例 |
| `stop_instance` | 停止指定实例 |
| `restart_instance` | 重启指定实例 |
| `get_server_status` | 获取模拟器全局状态 |

### 数据接口工具

| 工具名称 | 说明 |
|----------|------|
| `list_points` | 列出实例的所有测点及其当前值 |
| `read_point` | 读取单个测点的当前值 |
| `read_points` | 批量读取多个测点的当前值 |
| `write_point` | 写入单个测点的值 |
| `write_points` | **【核心】** 批量写入多个测点的值 |
| `config_auto_change` | 配置测点的自动变化策略 |
| `batch_config_auto_change` | 批量配置多个测点的自动变化策略 |
| `get_auto_change` | 查看测点的自动变化配置 |
| `delete_auto_change` | 删除测点的自动变化配置 |
| `export_auto_changes` | 导出实例所有自动变化配置为 CSV |
| `import_auto_changes` | 从 CSV 内容导入自动变化配置 |
| `upload_csv` | 上传 CSV 时间序列文件（用于 CSV 回放） |
| `list_csv_files` | 列出实例可用的 CSV 回放文件 |
| `config_csv_replay` | **【核心】** 配置 CSV 多测点同步回放（一键设置文件/时间/映射） |
| `upload_file` | 上传 .xlsx 点表文件 |
| `export_points_csv` | 导出实例所有测点实时数据为 CSV |
| `update_qds` | 更新测点的品质描述 QDS |

### 接口测试工具 (API Proxy)

| 工具名称 | 说明 |
|----------|------|
| `proxy_list_collections` | 获取所有 API 接口集合/请求列表 |
| `proxy_create_collection` | 创建新的 API 接口（request）或文件夹（folder），可设置 method/url/headers/body/前置脚本/后置脚本 |
| `proxy_update_collection` | 修改 API 接口的 URL/method/headers/body/前置脚本/后置脚本 |
| `proxy_delete_collection` | 删除指定 API 接口或文件夹 |
| `proxy_execute_request` | 执行 HTTP 请求代理，发送请求并返回响应状态码、响应头、响应体、耗时 |
| `proxy_list_environments` | 获取所有环境变量列表及当前激活的环境 ID |
| `proxy_create_environment` | 创建新的环境变量组，可设置多个变量键值对 |
| `proxy_update_environment` | 更新环境变量的值或名称 |
| `proxy_activate_environment` | 激活指定环境变量组，激活后执行接口时自动注入环境变量 |
| `proxy_delete_environment` | 删除指定环境变量组 |

### 全局工具

| 工具名称 | 说明 |
|----------|------|
| `list_files` | 列出 config 目录下所有 .xlsx 点表文件 |
| `get_protocols` | 查询模拟器支持的协议类型 |

## 使用示例

### 1. 配置自动变化策略

```python
# 使用 MCP 工具配置测点自动变化
config_auto_change(
    instance_id="inst-001",
    ioa=16385,
    strategy="increment",
    enabled=True,
    params='{"start_value":0,"step":1,"period_ms":1000,"max_value":100}'
)
```

支持的策略类型：
- `increment` - 递增
- `random` - 随机
- `csv` - CSV 回放
- `max` - 取大
- `min` - 取小
- `soc` - SOC 计算
- `energy` - 电量计算
- `aofollow` - AO 关联
- `apiupdate` - 接口更新
- `manual` - 手动
- `custom` - 自定义公式

### 2. 批量写入测点

```python
# 一次写入多个测点，模拟真实设备同一时刻上报数据
write_points(
    instance_id="inst-001",
    points=[
        {"ioa": 16385, "value": 235.5},
        {"ioa": 16386, "value": 236.0},
        {"ioa": 16387, "bool_value": True}
    ]
)
```

### 3. 上传点表文件

```python
# 上传 .xlsx 点表文件（文件内容需 base64 编码）
upload_file(
    filename="固定验证-关口表.xlsx",
    content_base64="UEsFBgAAAA..."
)
```

### 4. CSV 多测点同步回放

```python
# 上传多列 CSV 文件
upload_csv(
    instance_id="a1b2c3d4e5f6",
    csv_content="""time,母线电压,线路电流,有功功率
0,220.0,5.2,1144.0
1000,221.5,5.3,1173.9
2000,219.8,5.1,1120.9"""
)

# 一次调用配置全部测点映射
config_csv_replay(
    instance_id="a1b2c3d4e5f6",
    csv_file="replay_data.csv",
    time_format="relative",
    time_unit="ms",
    mappings=[
        {"column": 1, "ioa": 16385},
        {"column": 2, "ioa": 16386},
        {"column": 3, "ioa": 16387},
    ]
)
```

### 5. 接口测试：发送 HTTP 请求

```python
# 通过 GridSim 代理发送 GET 请求
proxy_execute_request(
    method="GET",
    url="https://api.example.com/data",
    headers='{"Authorization": "Bearer token123"}',
    timeout=30
)
```

### 6. 接口测试：创建并管理 API 集合

```python
# 创建文件夹
proxy_create_collection(
    name="电力系统API",
    type="folder"
)

# 在文件夹下创建接口
proxy_create_collection(
    name="获取实时数据",
    type="request",
    method="GET",
    url="{{base_url}}/api/realtime",
    headers='{"Content-Type": "application/json"}',
    pre_script="console.log('前置脚本执行')",
    test_script="pm.response.to.have.status(200)",
    parent_id="req-1717488000000"
)

# 修改接口的 URL 和请求体
proxy_update_collection(
    id="req-1717488000123",
    url="{{base_url}}/api/realtime/v2",
    body='{"device_id": "PV-001"}',
    method="POST"
)

# 删除接口
proxy_delete_collection(id="req-1717488000123")
```

### 7. 接口测试：管理环境变量

```python
# 创建环境
proxy_create_environment(
    name="测试环境",
    variables='{"base_url": "http://10.65.99.13:8989", "token": "dev-token-abc"}'
)

# 激活环境（后续请求自动注入 {{base_url}} 等变量）
proxy_activate_environment(id="env-1717488000000")

# 修改变量值
proxy_update_environment(
    id="env-1717488000000",
    name="测试环境",
    variables='{"base_url": "http://10.65.99.14:8989", "token": "prod-token-xyz"}'
)

# 删除环境
proxy_delete_environment(id="env-1717488000000")
```

## 一键安装包

使用项目根目录的 `iec104-autotester-pack.tar.gz` 快速部署：

```bash
# 解压
tar -xzf iec104-autotester-pack.tar.gz

# 进入目录
cd iec104-autotester-pack

# 查看内容
ls -la
```

包内包含：
- `gridsim` - IEC104 模拟器主程序
- `mcp-server` - MCP 服务器程序
- `config/` - 配置目录
- `samples/` - 示例点表

## 故障排除

### 连接失败

```
Error: dial tcp connection refused
```

检查：
1. IEC104 模拟器是否已启动
2. `-simulator` 参数地址是否正确
3. 模拟器 HTTP 端口是否可达

### 测点写入失败

```
错误: IOA xxx not found
```

检查：
1. 测点 IOA 是否存在于点表中
2. 实例是否已启动

### MCP 工具无响应

1. 检查模拟器日志
2. 确认点表文件已正确加载
3. 验证实例状态为 running

## 版本信息

- GridSim: 3.0.0+
- MCP Server: 1.3.0
- 工具总数: 38（原有28个 + 新增10个接口测试工具）
