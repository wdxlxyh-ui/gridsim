#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
分析API文档结构，找出重复章节
"""

import re
from collections import defaultdict

def analyze_document_structure(filepath):
    """分析文档结构"""
    
    with open(filepath, 'r', encoding='utf-8') as f:
        lines = f.readlines()
    
    print(f"文档总行数: {len(lines)}")
    
    # 查找所有章节标题
    chapters = []
    for i, line in enumerate(lines):
        line = line.strip()
        # 查找 ## 开头的章节标题
        if line.startswith('## ') and not line.startswith('###'):
            chapters.append((i, line))
        # 查找 # 开头的标题
        elif line.startswith('# ') and not line.startswith('##'):
            chapters.append((i, line))
    
    print("\n=== 文档章节结构 ===")
    for idx, (line_num, title) in enumerate(chapters):
        print(f"{idx:3d}. 行{line_num:4d}: {title}")
    
    # 分析重复的章节
    chapter_counts = defaultdict(list)
    for line_num, title in chapters:
        # 提取章节编号
        match = re.search(r'#+\s*(\d+(\.\d+)*)', title)
        if match:
            chapter_num = match.group(1)
            chapter_counts[chapter_num].append((line_num, title))
    
    print("\n=== 重复章节分析 ===")
    for chapter_num, occurrences in chapter_counts.items():
        if len(occurrences) > 1:
            print(f"\n章节 {chapter_num} 重复 {len(occurrences)} 次:")
            for i, (line_num, title) in enumerate(occurrences):
                print(f"  第{i+1}次: 行{line_num} - {title}")
    
    # 找到可能的问题区域
    print("\n=== 建议清理的区域 ===")
    for i in range(len(chapters)-1):
        curr_line, curr_title = chapters[i]
        next_line, next_title = chapters[i+1]
        
        # 如果两个相邻章节的编号相同或连续编号有问题
        curr_match = re.search(r'#+\s*(\d+(\.\d+)*)', curr_title)
        next_match = re.search(r'#+\s*(\d+(\.\d+)*)', next_title)
        
        if curr_match and next_match:
            curr_num = curr_match.group(1)
            next_num = next_match.group(1)
            
            # 检查编号连续性
            try:
                curr_int = float(curr_num) if '.' in curr_num else int(curr_num)
                next_int = float(next_num) if '.' in next_num else int(next_num)
                
                # 如果编号跳跃太大或重复
                if curr_int == next_int:
                    print(f"重复章节: 行{curr_line} '{curr_title}' 和 行{next_line} '{next_title}'")
                elif next_int - curr_int > 1:
                    print(f"编号跳跃: 从 {curr_num} 跳到 {next_num}")
                    
            except ValueError:
                pass
    
    return chapters, lines

def find_problem_areas(chapters, lines):
    """找出文档中的问题区域"""
    
    print("\n=== 具体问题区域 ===")
    
    # 查找"录制接口"出现的位置
    recording_locations = []
    for i, line in enumerate(lines):
        if '录制接口' in line and '##' in line:
            recording_locations.append((i, line.strip()))
    
    if recording_locations:
        print("\n'录制接口'章节位置:")
        for line_num, line in recording_locations:
            print(f"  行{line_num}: {line}")
    
    # 查找"接口测试"出现的位置
    proxy_locations = []
    for i, line in enumerate(lines):
        if '接口测试' in line and '##' in line:
            proxy_locations.append((i, line.strip()))
    
    if proxy_locations:
        print("\n'接口测试'章节位置:")
        for line_num, line in proxy_locations:
            print(f"  行{line_num}: {line}")
    
    # 查找OpenAPI差异说明位置
    openapi_locations = []
    for i, line in enumerate(lines):
        if 'OpenAPI 差异说明' in line and '##' in line:
            openapi_locations.append((i, line.strip()))
    
    if openapi_locations:
        print("\n'OpenAPI差异说明'章节位置:")
        for line_num, line in openapi_locations:
            print(f"  行{line_num}: {line}")

if __name__ == "__main__":
    input_file = r"docs\API-文档.md"
    
    print(f"正在分析文档: {input_file}")
    chapters, lines = analyze_document_structure(input_file)
    find_problem_areas(chapters, lines)
    
    print("\n=== 分析完成 ===")