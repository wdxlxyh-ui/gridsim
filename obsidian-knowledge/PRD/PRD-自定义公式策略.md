---
title: PRD-自定义公式策略
tags:
  - prd
  - custom-formula
  - strategy
aliases:
  - Custom Formula PRD
created: 2026-05-23
---

# PRD: 自定义公式自动变化策略

> 版本：v2.2.0-dev

## 需求概述

新增第 11 种自动变化策略——自定义公式。用户选择多个关联测点，通过按钮组合构造四则运算公式，按周期计算结果写入目标测点。

## 用户故事

> 作为一个变电站自动化测试人员，我需要在模拟器中配置一个测点，其值由其他多个测点的值通过自定义公式计算得出（例如：母线电压 = 线路1电压 + 线路2电压 - 线路3电压），以便在 IEC104 客户端观察联动变化效果。

## 参数定义

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `custom_ioas` | string | 是 | 关联 IOA 列表，分号分隔，2~50 个 |
| `custom_formula` | string | 是 | 公式字符串 |
| `period_ms` | int | 是 | 计算周期（≥100ms） |

## 公式约束

- 支持 `+` `-` `*` `/` `(` `)`
- `{n}` 占位符对应第 n+1 个关联 IOA
- 递归下降解析器
- 除零保护
- 公式语法错误时跳过并记录日志

## API 变更

```json
{
  "strategy": "custom",
  "enabled": true,
  "params": {
    "custom_ioas": "16385;16386;16387",
    "custom_formula": "{0}+{1}-{2}",
    "period_ms": 1000
  }
}
```

## CSV 导入/导出

策略代码 `11`，A 列=custom_ioas，B 列=custom_formula，C 列=period_ms。

---

相关笔记：[[PRD/PRD 索引|PRD 索引]]、[[功能特性/策略-自定义公式|自定义公式策略]]
