#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
创建最终的API文档
"""

def create_final_document():
    """创建最终的API文档"""
    
    # 读取修复后的文档
    with open(r"docs\API-文档-final.md", 'r', encoding='utf-8') as f:
        lines = f.readlines()
    
    print(f"修复后文档行数: {len(lines)}")
    
    # 找到第14章结束的位置
    chapter_14_end = None
    for i, line in enumerate(lines):
        if '## 14. AI 友好接口 (v3.0.1 新增)' in line:
            # 找到第14章结束的位置（在幂等性部分之后）
            for j in range(i+1, len(lines)):
                if lines[j].startswith('---\n'):
                    chapter_14_end = j
                    print(f"第14章结束于行: {j}")
                    break
            break
    
    if chapter_14_end is None:
        print("错误：找不到第14章结束位置")
        return False
    
    # 找到第15章开始的位置
    chapter_15_start = None
    for i in range(chapter_14_end, len(lines)):
        if '## 15. 状态码与错误处理' in lines[i]:
            chapter_15_start = i
            print(f"第15章开始于行: {i}")
            break
    
    if chapter_15_start is None:
        print("错误：找不到第15章开始位置")
        return False
    
    # 构建OpenAPI差异说明章节
    openapi_chapter = '''---

## 15. OpenAPI 差异说明

### 15.1 当前 OpenAPI 规范问题

实时 `/openapi.json` 规范存在以下不一致：

| 差异点 | 实际路由源码 | 实时 OpenAPI (3.0.3) |
|---|---|---|
| **API 版本** | `3.2.1-dev+9.64e7f61` | `3.1.0`（错误） |
| **路径数量** | 80+ 条完整路由 | 仅 30 条路径（不完整） |
| **微电网路径** | `/api/v1/instances/{id}/microgrid/*` | `/api/v1/microgrid/*`（缺少实例 ID） |
| **认证中间件** | 未接入路由（可直连） | 标记为需要 Bearer Token |
| **状态码定义** | 源码定义两种错误格式 | 仅旧式错误格式 |
| **录制回放** | 仅录制、无回放接口 | 包含回放接口（未实现） |

### 15.2 推荐做法

1. **以源码为真源**：路由定义在 `main.go`、`detail/handler.go`、`microgrid/handler.go`、`py_microgrid_handler.go` 等文件
2. **交叉验证**：实际调用确认参数和响应格式
3. **忽略错误版本号**：OpenAPI 的 `info.version` 未自动更新，以部署版本为准

---

'''
    
    # 构建最终的文档
    # 第1部分：从开始到第14章结束
    part1 = lines[:chapter_14_end + 1]
    
    # 第2部分：OpenAPI差异说明章节
    part2 = [openapi_chapter]
    
    # 第3部分：从第15章开始到结尾，但需要重新编号
    part3_lines = lines[chapter_15_start:]
    
    # 重新编号第15章及之后的章节
    part3 = []
    for line in part3_lines:
        # 重新编号章节
        if line.startswith('## 15.'):
            line = line.replace('## 15.', '## 16.')
        elif line.startswith('## 16.'):
            line = line.replace('## 16.', '## 17.')
        elif line.startswith('## 17.'):
            line = line.replace('## 17.', '## 18.')
        elif line.startswith('## 18.'):
            line = line.replace('## 18.', '## 19.')
        elif line.startswith('### 15.'):
            line = line.replace('### 15.', '### 16.')
        elif line.startswith('### 16.'):
            line = line.replace('### 16.', '### 17.')
        elif line.startswith('### 17.'):
            line = line.replace('### 17.', '### 18.')
        elif line.startswith('### 18.'):
            line = line.replace('### 18.', '### 19.')
        part3.append(line)
    
    # 合并所有部分
    final_lines = part1 + part2 + part3
    
    print(f"最终文档行数: {len(final_lines)}")
    
    # 更新目录
    updated_toc = '''## 目录

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
15. [OpenAPI 差异说明](#15-openapi-差异说明)
16. [状态码与错误处理](#16-状态码与错误处理)
17. [速率限制与配额](#17-速率限制与配额)
18. [最佳实践与示例](#18-最佳实践与示例)
19. [常见问题 (FAQ)](#19-常见问题-faq)

'''
    
    # 替换目录
    for i, line in enumerate(final_lines):
        if line.strip() == '## 目录':
            # 找到目录结束的位置
            for j in range(i+1, len(final_lines)):
                if final_lines[j].startswith('---'):
                    # 替换目录
                    final_lines[i] = '## 目录\n'
                    final_lines[i+1:j] = [updated_toc]
                    print("已更新目录")
                    break
            break
    
    # 保存最终文档
    output_file = r"docs\GridSim-API-文档-3.2.1-dev.md"
    with open(output_file, 'w', encoding='utf-8') as f:
        f.writelines(final_lines)
    
    print(f"已保存最终文档到: {output_file}")
    
    # 验证最终文档结构
    print("\n=== 最终文档章节结构 ===")
    chapter_count = 0
    for i, line in enumerate(final_lines):
        if line.startswith('## ') and not line.startswith('###'):
            chapter_count += 1
            print(f"{chapter_count:2d}. 行{i:4d}: {line.strip()}")
        elif line.startswith('# ') and not line.startswith('##'):
            chapter_count += 1
            print(f"{chapter_count:2d}. 行{i:4d}: {line.strip()}")
    
    return True

if __name__ == "__main__":
    print("正在创建最终的API文档...")
    if create_final_document():
        print("\n最终文档创建完成！")
    else:
        print("\n最终文档创建失败！")