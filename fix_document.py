#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
修复API文档，删除重复章节
"""

def fix_document(input_file, output_file):
    """修复文档结构"""
    
    with open(input_file, 'r', encoding='utf-8') as f:
        lines = f.readlines()
    
    print(f"原始文件行数: {len(lines)}")
    
    # 找到第15章"OpenAPI差异说明"的结束位置
    # 我们保留第15章，删除后面的重复章节
    chapter_15_start = None
    chapter_15_end = None
    
    for i, line in enumerate(lines):
        if '## 15. OpenAPI 差异说明' in line:
            chapter_15_start = i
            print(f"第15章开始于行: {i}")
            break
    
    if chapter_15_start is None:
        print("错误：找不到第15章")
        return False
    
    # 查找第15章结束的位置（下一个章节开始）
    for i in range(chapter_15_start + 1, len(lines)):
        if lines[i].startswith('---\n'):
            # 检查后面的行是否开始新章节
            for j in range(i+1, min(i+5, len(lines))):
                if lines[j].startswith('## ') or lines[j].startswith('# '):
                    chapter_15_end = i
                    print(f"第15章结束于行: {i} (在 '{lines[j].strip()}' 之前)")
                    break
            if chapter_15_end:
                break
    
    if chapter_15_end is None:
        # 如果找不到明显的结束，就用文件结尾
        chapter_15_end = len(lines) - 1
        print(f"第15章结束于文件末尾: {chapter_15_end}")
    
    # 现在我们需要找到正确的文档结尾部分
    # 查找第18章"常见问题 (FAQ)"开始的位置
    chapter_18_start = None
    for i in range(chapter_15_end, len(lines)):
        if '## 18. 常见问题 (FAQ)' in lines[i]:
            chapter_18_start = i
            print(f"第18章开始于行: {i}")
            break
    
    if chapter_18_start is None:
        print("错误：找不到第18章")
        return False
    
    # 构建修复后的文档
    # 第1部分：从开始到第15章结束
    part1 = lines[:chapter_15_end + 1]
    
    # 第2部分：从第18章开始到结尾
    part2 = lines[chapter_18_start:]
    
    # 合并两部分
    fixed_lines = part1 + ['\n'] + part2
    
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
    output_file = r"docs\API-文档-fixed.md"
    
    print(f"正在修复文档: {input_file}")
    if fix_document(input_file, output_file):
        print("\n文档修复完成！")
    else:
        print("\n文档修复失败！")