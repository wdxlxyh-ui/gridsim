---
title: PRD-多规约支持
tags:
  - prd
  - protocol
  - modbus
aliases:
  - Multi-Protocol PRD
created: 2026-05-23
---

# PRD: 多规约支持 — Modbus TCP 扩展

## 背景

当前 GridSim v2.3.0 仅支持 IEC104 规约。目标是在不影响现有 IEC104 的前提下扩展 Modbus TCP 支持。

## 核心需求

| 编号 | 需求 | 优先级 |
|------|------|--------|
| R1 | 新建实例时选择规约类型 | P0 |
| R2 | 点表扩展 Modbus 字段（功能码、寄存器地址） | P0 |
| R3 | Modbus TCP 服务端 | P0 |
| R4 | 向前兼容 | P0 |
| R5 | 详情页适配 | P0 |
| R6 | Excel 点表模板 | P0 |

## 点表扩展列

| 列 | 表头 | Modbus TCP |
|----|------|------------|
| H | function-code | 功能码（01-06,15,16） |
| I | register-address | 寄存器地址（0-65535） |
| J | byte-order | 字节序（ABCD/CDAB/BADC/DCBA） |

## 验收标准

| 场景 | 条件 |
|------|------|
| 创建 Modbus 实例 | 选择规约→上传点表→启动→Modbus Poll 可连接读取 |
| 旧 IEC104 实例 | 升级后原有实例正常运行 |
| 点表兼容性 | IEC104 点表（无 H/I/J 列）正常加载 |
| 详情页 | Modbus 实例操作与 IEC104 一致 |

---

相关笔记：[[PRD/PRD 索引|PRD 索引]]、[[协议支持/多规约架构|多规约架构]]
