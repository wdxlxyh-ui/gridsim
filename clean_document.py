#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
清理API文档重复内容
"""

import re

def clean_document(input_file, output_file):
    """清理文档中的重复内容"""
    
    with open(input_file, 'r', encoding='utf-8') as f:
        content = f.read()
    
    print(f"原始文件长度: {len(content)} 字符")
    
    # 找到第8章结束的位置（在"p_edc"之后）
    chapter_8_end = content.find('"pulse_status": "INACTIVE"\n}\n```')
    if chapter_8_end == -1:
        print("错误：找不到第8章结束位置")
        return False
    
    chapter_8_end += len('"pulse_status": "INACTIVE"\n}\n```')
    print(f"第8章结束位置: {chapter_8_end}")
    
    # 找到正确的第9章开始位置
    chapter_9_start = content.find('内置 Postman 风格的接口测试工具。')
    if chapter_9_start == -1:
        print("错误：找不到第9章开始位置")
        return False
    
    # 从第9章开始位置往前找"## 9."标题
    chapter_9_header_start = content.rfind('\n## 9.', 0, chapter_9_start)
    if chapter_9_header_start == -1:
        chapter_9_header_start = content.rfind('\n## 9', 0, chapter_9_start)
    
    print(f"正确的第9章标题位置: {chapter_9_header_start}")
    print(f"正确的第9章内容位置: {chapter_9_start}")
    
    # 提取第8章之前的内容
    part1 = content[:chapter_8_end]
    
    # 添加分隔符
    part1 += "\n\n---\n\n"
    
    # 从正确位置提取第9章及之后的内容
    # 从第9章标题开始
    part2 = content[chapter_9_header_start:]
    
    # 合并两部分
    cleaned_content = part1 + part2
    
    print(f"清理后文件长度: {len(cleaned_content)} 字符")
    
    # 保存清理后的文档
    with open(output_file, 'w', encoding='utf-8') as f:
        f.write(cleaned_content)
    
    print(f"已保存清理后的文档到: {output_file}")
    return True

if __name__ == "__main__":
    input_file = r"docs\API-文档.md"
    output_file = r"docs\API-文档-cleaned.md"
    
    if clean_document(input_file, output_file):
        print("文档清理完成！")
    else:
        print("文档清理失败！")