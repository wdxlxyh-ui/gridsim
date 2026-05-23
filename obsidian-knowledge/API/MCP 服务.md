---
title: MCP 服务
tags:
  - api
  - mcp
  - tools
aliases:
  - MCP Server
  - Model Context Protocol
created: 2026-05-23
---

# MCP 服务

> 版本: 1.1 | 工具总数: 28

## 概述

MCP (Model Context Protocol) 服务器提供 IEC 104 模拟器的程序化控制接口，支持 AI 助手直接控制模拟器。

## 快速开始

```bash
go build -o bin/mcp-server ./cmd/mcp-server/
./bin/mcp-server -simulator http://localhost:8989 -mode both
```

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-simulator` | http://localhost:8989 | 模拟器 HTTP 地址 |
| `-mode` | both | instance / data / both |

## 实例管理工具（9 个）

| 工具 | 说明 |
|------|------|
| `list_instances` | 列出所有实例 |
| `get_instance` | 获取实例详情 |
| `create_instance` | 创建实例 |
| `update_instance` | 更新实例 |
| `delete_instance` | 删除实例 |
| `start_instance` | 启动实例 |
| `stop_instance` | 停止实例 |
| `restart_instance` | 重启实例 |
| `get_server_status` | 获取全局状态 |

## 数据接口工具（17 个）

| 工具 | 说明 |
|------|------|
| `list_points` | 列出所有测点 |
| `read_point` | 读取单个测点 |
| `read_points` | 批量读取测点 |
| `write_point` | 写入单个测点 |
| `write_points` | **【核心】** 批量写入测点 |
| `config_auto_change` | 配置自动变化策略 |
| `batch_config_auto_change` | 批量配置自动变化 |
| `get_auto_change` | 查看自动变化配置 |
| `delete_auto_change` | 删除自动变化配置 |
| `export_auto_changes` | 导出自动变化配置 |
| `import_auto_changes` | 导入自动变化配置 |
| `upload_csv` | 上传 CSV 文件 |
| `list_csv_files` | 列出 CSV 文件 |
| `config_csv_replay` | **【核心】** 配置 CSV 回放 |
| `upload_file` | 上传点表文件 |
| `export_points_csv` | 导出测点 CSV |
| `update_qds` | 更新品质描述 |

## 全局工具（2 个）

| 工具 | 说明 |
|------|------|
| `list_files` | 列出 .xlsx 点表文件 |
| `get_protocols` | 查询支持协议类型 |

## 使用示例

```python
# 批量写入
write_points(
    instance_id="inst-001",
    points=[
        {"ioa": 16385, "value": 235.5},
        {"ioa": 16386, "value": 236.0},
        {"ioa": 16387, "bool_value": True}
    ]
)

# CSV 多测点回放
config_csv_replay(
    instance_id="a1b2c3d4e5f6",
    csv_file="replay_data.csv",
    time_format="relative",
    time_unit="ms",
    mappings=[
        {"column": 1, "ioa": 16385},
        {"column": 2, "ioa": 16386},
    ]
)
```

## 支持的策略类型

`increment` `random` `csv` `max` `min` `soc` `energy` `aofollow` `apiupdate` `manual` `custom`

---

相关笔记：[[API/REST API 文档|REST API 文档]]、[[功能特性/自动变化策略|自动变化策略]]
