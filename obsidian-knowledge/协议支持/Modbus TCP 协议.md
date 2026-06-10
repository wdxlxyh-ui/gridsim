---
title: Modbus TCP 协议
tags:
  - modbus
  - protocol
aliases:
  - Modbus TCP
created: 2026-05-23
---

# Modbus TCP 协议

> 自 v2.4.0 起支持

## 与 IEC104 对比

| 维度 | IEC104 | Modbus TCP |
|------|--------|------------|
| 地址标识 | IOA（信息体地址） | Register Address / Coil Address |
| 功能标识 | ASDU TypeID | Function Code |
| 数据类型 | AI/DI/PI/DO/AO | Holding/Input Register, Coil, DI |
| 端口 | 自定义 | 默认 502 |
| 字节序 | IEEE 754 float | 可配置 (ABCD/CDAB/BADC/DCBA) |
| 并发客户端 | 1 | ≥10 |

## 点表扩展列

| 列 | 表头 | 说明 |
|----|------|------|
| A-G | 标准列 | 同 IEC104 |
| H | function-code | Modbus 功能码（01-06,15,16） |
| I | register-address | Modbus 寄存器地址（0-65535） |
| J | byte-order | 字节序（默认 ABCD） |

## 功能码与测点映射

| 测点类型 | 读功能码 | 写功能码 | Modbus 区域 |
|----------|----------|----------|-------------|
| AI（遥测） | 04 Read Input Registers | — | 3x Input Registers |
| DI（遥信） | 02 Read Discrete Inputs | — | 1x Discrete Inputs |
| PI（遥脉） | 04 Read Input Registers | — | 3x Input Registers (32bit) |
| DO（遥控） | 01 Read Coils | 05/15 Write Coil(s) | 0x Coils |
| AO（遥调） | 03 Read Holding Registers | 06/16 Write Register(s) | 4x Holding Registers |

## 数据类型转换

Float32/Int32 与 2×16-bit registers 之间转换，支持：
- **ABCD** — 大端序（默认）
- **CDAB** — 字交换
- **BADC** — 字节交换
- **DCBA** — 小端序

## 向后兼容

- 已有 IEC104 实例的 JSON 配置无需修改（protocol 为空 → 默认 iec104）
- 旧版 Excel 点表无 H/I/J 列 → 零值回退
- 旧版前端不发送 protocol → 后端默认 iec104

---

相关笔记：[[协议支持/IEC104 协议|IEC104 协议]]、[[协议支持/多规约架构|多规约架构]]
