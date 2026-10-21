#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
修复API文档版本2：保留正确章节，删除重复章节
"""

def fix_document_v2(input_file, output_file):
    """修复文档结构，保留正确的第15-17章"""
    
    with open(input_file, 'r', encoding='utf-8') as f:
        lines = f.readlines()
    
    print(f"原始文件行数: {len(lines)}")
    
    # 找到第14章"AI友好接口"的结束位置
    chapter_14_end = None
    for i, line in enumerate(lines):
        if '## 14. AI 友好接口 (v3.0.1 新增)' in line:
            # 查找第14章结束的位置
            for j in range(i+1, len(lines)):
                if lines[j].startswith('---\n'):
                    # 检查是否是新章节的开始
                    for k in range(j+1, min(j+5, len(lines))):
                        if lines[k].startswith('## ') or lines[k].startswith('# '):
                            chapter_14_end = j
                            print(f"第14章结束于行: {j}")
                            break
                    if chapter_14_end:
                        break
            break
    
    if chapter_14_end is None:
        print("错误：找不到第14章结束位置")
        return False
    
    # 找到第15章"状态码与错误处理"的开始位置
    chapter_15_start = None
    for i in range(chapter_14_end, len(lines)):
        if '## 15. 状态码与错误处理' in lines[i]:
            chapter_15_start = i
            print(f"第15章开始于行: {i}")
            break
    
    if chapter_15_start is None:
        print("错误：找不到第15章开始位置")
        return False
    
    # 找到第18章"常见问题"的开始位置
    chapter_18_start = None
    for i in range(chapter_15_start, len(lines)):
        if '## 18. 常见问题 (FAQ)' in lines[i]:
            chapter_18_start = i
            print(f"第18章开始于行: {i}")
            break
    
    if chapter_18_start is None:
        print("错误：找不到第18章开始位置")
        return False
    
    # 构建修复后的文档
    # 第1部分：从开始到第14章结束
    part1 = lines[:chapter_14_end + 1]
    
    # 第2部分：从第15章开始到第18章之前
    part2 = lines[chapter_15_start:chapter_18_start]
    
    # 第3部分：从第18章开始到结尾
    part3 = lines[chapter_18_start:]
    
    # 合并三部分
    fixed_lines = part1 + ['\n'] + part2 + ['\n'] + part3
    
    print(f"修复后文件行数: {len(fixed_lines)}")
    print(f"删除了 {len(lines) - len(fixed_lines)} 行重复内容")
    
    # 保存修复后的文档
    with open(output_file, 'w', encoding='utf-8') as f:
        f.writelines(fixed_lines)
    
    print(f"已保存修复后的文档到: {output_file}")
    
    # 验证修复结果
    print("\n=== 修复后章节结构 ===")
    chapter_count = 0
    for i, line in enumerate(fixed_lines):
        if line.startswith('## ') and not line.startswith('###'):
            chapter_count += 1
            print(f"{chapter_count:2d}. 行{i:4d}: {line.strip()}")
        elif line.startswith('# ') and not line.startswith('##'):
            chapter_count += 1
            print(f"{chapter_count:2d}. 行{i:4d}: {line.strip()}")
    
    return True

if __name__ == "__main__":
    input_file = r"docs\API-文档.md"
    output_file = r"docs\API-文档-final.md"
    
    print(f"正在修复文档: {input_file}")
    if fix_document_v2(input_file, output_file):
        print("\n文档修复完成！")
    else:
        print("\n文档修复失败！")