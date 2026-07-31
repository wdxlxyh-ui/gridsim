#!/usr/bin/env python3
"""Generate GridSim v3.2.1 user manual HTML (UI操作手册, no API/protocol details)."""
import re, os

DOCS_DIR = os.path.dirname(os.path.abspath(__file__))
OLD_FILE = os.path.join(DOCS_DIR, '用户手册-GridSim操作指南.html')
OUT_FILE = os.path.join(DOCS_DIR, 'GridSim-v3.2.1-用户操作手册.html')

with open(OLD_FILE, 'r', encoding='utf-8') as f:
    old_content = f.read()
head_match = re.search(r'(<head>.*?</head>)', old_content, re.DOTALL)
head_html = head_match.group(1).replace(
    'GridSim 用户操作手册 — IEC104 模拟器 v2.5.3',
    'GridSim v3.2.1 — 用户操作手册')

def img(cap):
    return f'<div class="screenshot-block"><div style="background:#f1f5f9;border:2px dashed #cbd5e1;border-radius:8px;padding:60px 20px;text-align:center;color:#64748b;font-style:italic;font-size:14px;">[此处插入截图: {cap}]</div><div class="cap"><strong>{cap}</strong></div></div>'

sidebar = """
<aside class="sidebar">
  <div class="sidebar-brand">
    <div class="title">GridSim</div>
    <div class="sub">用户操作手册</div>
    <span class="version">v3.2.1</span>
  </div>
  <nav>
    <a href="#overview" class="active"><span class="icon">📋</span> 概述</a>
    <div class="nav-section">🚀 部署</div>
    <a href="#deploy"><span class="icon">📦</span> 部署与启动</a>
    <div class="nav-section">🖥️ Web UI 操作</div>
    <a href="#dashboard"><span class="icon">📊</span> 仪表盘</a>
    <a href="#config"><span class="icon">⚙️</span> 配置管理</a>
    <a href="#monitor"><span class="icon">📡</span> 运行监控</a>
    <a href="#detail"><span class="icon">📈</span> 实例详情</a>
    <a href="#strategies"><span class="icon">🔄</span> 自动变化策略</a>
    <a href="#csv-replay"><span class="icon">📁</span> CSV 回放</a>
    <a href="#trend"><span class="icon">📉</span> 趋势图</a>
    <div class="nav-section">🔋 Python 微电网</div>
    <a href="#py-create"><span class="icon">➕</span> 创建与配置</a>
    <a href="#py-run"><span class="icon">▶️</span> 启动与组态</a>
    <a href="#py-control"><span class="icon">🎮</span> 设备控制</a>
    <a href="#py-export"><span class="icon">📄</span> 导出点表</a>
    <a href="#py-curves"><span class="icon">📁</span> 功率曲线</a>
    <div class="nav-section">⚡ 微电网仿真</div>
    <a href="#microgrid"><span class="icon">🔌</span> Go 微电网</a>
    <div class="nav-section">🔧 其他</div>
    <a href="#faq"><span class="icon">❓</span> 常见问题</a>
  </nav>
</aside>"""

body = f"""<div class="main">

<h1 id="overview">GridSim 用户操作手册</h1>
<p><strong>GridSim</strong> 是一款多协议电力系统仿真工具，支持 IEC104、Modbus TCP、微电网仿真和 Python 微电网多实例仿真。通过 Web 管理界面完成所有操作。</p>
<div class="info-box info"><div class="label">💡 适用版本</div>本文档面向 GridSim v3.2.1。默认账号 <code>admin</code> / <code>admin123</code></div>

<!-- ═══ 部署 ═══ -->
<h2 id="deploy">部署与启动</h2>
<h3>一键部署</h3>
<p>将安装包上传到服务器后执行：</p>
<pre>bash gridsim-install-v3.2.1-linux-amd64.sh</pre>
<p>脚本自动完成全部操作。部署完成后浏览器访问 <code>http://&lt;服务器IP&gt;:8989</code> 进入管理界面。</p>
<h3>手动启停</h3>
<p>启动：<code>./bin/start.sh</code>　停止：<code>./bin/stop.sh</code>　重启：<code>./bin/restart.sh</code></p>

<!-- ═══ 仪表盘 ═══ -->
<h2 id="dashboard">仪表盘</h2>
{img('仪表盘界面')}
<p>首页展示所有实例的状态卡片，每张卡片显示：</p>
<ul>
<li>运行状态（运行中 / 已停止 / 错误）</li>
<li>协议类型标签（IEC104 / Modbus TCP / 微电网 / Python微电网）</li>
<li>连接状态、测点数、运行时长</li>
<li>端口号（根据协议类型自动显示对应端口）</li>
</ul>
<p>点击卡片可快速进入对应实例的详情或仿真页面。</p>

<!-- ═══ 配置管理 ═══ -->
<h2 id="config">配置管理</h2>
{img('配置管理页面')}
<p>配置管理页面可以创建、编辑、启动、停止、删除实例。支持批量操作。</p>
<h3>创建实例</h3>
{img('创建实例向导')}
<p>点击"添加实例"，四步向导：</p>
<ol class="step-list">
<li><strong>规约与名称</strong>选择协议类型，填写实例名称</li>
<li><strong>网络配置</strong>根据协议填写端口等参数。Python 微电网填写 Modbus 端口（不能与其他实例重复）、轮询间隔、仿真起始时间</li>
<li><strong>点表与接口</strong>IEC104/Modbus 需上传 xlsx 点表；Python 微电网/微电网无需点表。可选启用 HTTP 接口</li>
<li><strong>确认</strong>检查配置摘要，IEC104/Modbus 可选"创建后立即启动"</li>
</ol>
<h3>编辑实例</h3>
<p>实例停止状态下点击"编辑"，可修改名称、端口、HTTP 配置等。规约类型不可修改。</p>

<!-- ═══ 运行监控 ═══ -->
<h2 id="monitor">运行监控</h2>
{img('运行监控界面')}
<p>实时监控所有实例状态。每张卡片显示端口、连接状态、测点数、运行时间、轮询/总召次数。可直接进行启动、停止、重启操作。</p>
"""

body += f"""
<!-- ═══ 实例详情 ═══ -->
<h2 id="detail">实例详情</h2>
{img('实例详情页')}
<p>IEC104/Modbus 实例点击"详情"进入。核心功能：</p>
<ul>
<li><strong>测点实时数据</strong>：表格展示所有测点当前值，支持 100ms~1000ms 刷新频率调节</li>
<li><strong>测点置数</strong>：AI 输入数值，DI 开关 ON/OFF，PI 输入整数</li>
<li><strong>自动变化策略</strong>：为每个测点配置自动计算策略</li>
<li><strong>CSV 多测点回放</strong>：上传 CSV 文件回放真实数据</li>
<li><strong>批量操作</strong>：勾选多个测点统一配置策略</li>
<li><strong>导出/导入</strong>：测点数据 CSV 导出、策略配置 JSON 导出/导入</li>
</ul>

<!-- ═══ 自动变化策略 ═══ -->
<h2 id="strategies">自动变化策略</h2>
{img('自动变化策略配置')}
<p>每个测点可独立配置以下 11 种策略：</p>
<table><thead><tr><th>#</th><th>策略名称</th><th>用途</th></tr></thead><tbody>
<tr><td>1</td><td>递增</td><td>值从起点按步长递增到上限后重置</td></tr>
<tr><td>2</td><td>随机</td><td>在指定范围内随机波动</td></tr>
<tr><td>3</td><td>CSV 回放</td><td>按 CSV 文件中的时序数据回放</td></tr>
<tr><td>4</td><td>MAX</td><td>取多个关联测点中的最大值</td></tr>
<tr><td>5</td><td>MIN</td><td>取多个关联测点中的最小值</td></tr>
<tr><td>6</td><td>SOC 计算</td><td>基于功率积分计算荷电状态</td></tr>
<tr><td>7</td><td>电量统计</td><td>功率积分累计电量</td></tr>
<tr><td>8</td><td>AO 关联</td><td>跟随控制指令值变化</td></tr>
<tr><td>9</td><td>接口更新</td><td>仅允许通过接口写入</td></tr>
<tr><td>10</td><td>手动</td><td>不自动计算，手动置数</td></tr>
<tr><td>11</td><td>自定义公式</td><td>四则运算公式，支持多测点关联</td></tr>
</tbody></table>
<p>操作：在详情页测点列表中，点击某测点的"策略"按钮，选择策略类型，填写参数，保存启用。</p>

<!-- ═══ CSV 回放 ═══ -->
<h2 id="csv-replay">CSV 多测点回放</h2>
{img('CSV 回放界面')}
<p>在详情页"CSV 回放"卡片区域操作：</p>
<ol class="step-list">
<li><strong>上传或选择 CSV 文件</strong>格式：第 1 列时间，后续列为各测点值</li>
<li><strong>列映射</strong>自动按列名匹配测点，也可手动指定列与 IOA 的对应关系</li>
<li><strong>选择时间模式</strong>相对时间（毫秒/秒）或绝对时间（hh:mm:ss）</li>
<li><strong>启动回放</strong>多测点共享同一时间基准，严格同步播放</li>
</ol>

<!-- ═══ 趋势图 ═══ -->
<h2 id="trend">趋势图</h2>
{img('趋势图页面')}
<p>选择多个测点后进入趋势页面，实时绘制曲线。支持：</p>
<ul>
<li>时间范围选择：5m / 15m / 30m / 1h / 2h</li>
<li>缩放拖拽、十字准线 tooltip</li>
<li>图例显隐控制</li>
<li>多线对比</li>
</ul>
"""

body += f"""
<!-- ═══ Python 微电网 ═══ -->
<h2 id="py-create">Python 微电网 — 创建与配置</h2>
<p>Python 微电网支持多实例并行，每个实例独立端口、独立进程。</p>
<h3>创建实例</h3>
<ol class="step-list">
<li><strong>添加实例</strong>配置管理 → 添加实例 → 规约选"Python微电网"</li>
<li><strong>网络配置</strong>填写 Modbus 端口（多实例不能重复）、轮询间隔、仿真起始时间</li>
<li><strong>确认创建</strong>实例为停止状态，需先配置设备</li>
</ol>
<h3>配置设备</h3>
<p>点击"Python仿真" → 切换到"配置管理" tab：</p>
{img('Python 微电网配置页面')}
<ul>
<li><strong>电表（Meter）</strong>：固定 1 台，自动汇总全站功率</li>
<li><strong>光伏（PV）</strong>：设置额定功率、出力模式（正弦/CSV）</li>
<li><strong>储能（BESS）</strong>：设置容量、充放电功率、初始 SOC</li>
<li><strong>充电桩（EV）</strong>：设置额定功率、充电时段</li>
<li><strong>负荷（Load）</strong>：设置基础功率、曲线模式</li>
</ul>
<p>编辑完成后点击"保存配置"。修改配置需先停止实例。</p>

<h2 id="py-run">Python 微电网 — 启动与组态</h2>
<p>配置完设备后在配置管理列表点击"启动"。</p>
{img('Python 组态图')}
<p>组态图动态显示：电网 → 母线 → 各设备，实时刷新功率值和 SOC。修改设备后重启即可看到更新。</p>

<h2 id="py-control">Python 微电网 — 设备控制</h2>
<p>运行中可在"设备控制"面板对各设备下发指令：</p>
<ul>
<li><strong>PV</strong>：修改限功率值</li>
<li><strong>BESS</strong>：下发充放电功率指令（正值放电，负值充电）</li>
<li><strong>EV</strong>：修改充电功率设定</li>
</ul>

<h2 id="py-export">Python 微电网 — 导出点表</h2>
<p>运行中实例点击"导出点表"按钮，下载 zip 压缩包。包内每个设备一个独立的 xlsx 文件，格式兼容 EnOS Modbus TCP 标准，可直接导入 Edge 创建访问模板。</p>
<div class="info-box info"><div class="label">💡 点表说明</div>数据类型 SW_INT（大端 4 字节整数），系数 0.01。功率和可控状态测点分配到高频采集组（100ms）。</div>

<h2 id="py-curves">Python 微电网 — 功率曲线</h2>
<p>在"配置管理" tab 底部"功率曲线管理"区域上传 CSV 文件。格式：第 1 列小时（0.25 步长），第 2 列功率(kW)。内置 pv_curve.csv 和 load_curve.csv 可直接使用。</p>

<!-- ═══ Go 微电网 ═══ -->
<h2 id="microgrid">微电网仿真（Go 原生）</h2>
{img('微电网拓扑编辑器')}
<p>Go 原生微电网通过拓扑编辑器配置设备（光伏/储能/负荷/充电桩），支持：</p>
<ul>
<li>SVG 拓扑组态图，实时功率流</li>
<li>设备参数表单配置</li>
<li>远方/本地控制模式</li>
<li>功率平衡计算</li>
</ul>

<!-- ═══ FAQ ═══ -->
<h2 id="faq">常见问题</h2>
<table><thead><tr><th>问题</th><th>解决方法</th></tr></thead><tbody>
<tr><td>Web 界面无法访问</td><td>确认服务已启动，防火墙放行 8989 端口</td></tr>
<tr><td>实例启动失败</td><td>检查端口是否被占用，查看 logs/output.log 日志</td></tr>
<tr><td>Python 实例启动超时</td><td>检查实例日志 config/py_instances/&lt;id&gt;/log/</td></tr>
<tr><td>端口冲突</td><td>各实例端口不能重复，修改后重试</td></tr>
<tr><td>外部设备无法连接 Modbus 端口</td><td>在服务器执行 <code>iptables -I INPUT 1 -p tcp --dport &lt;端口&gt; -j ACCEPT</code></td></tr>
<tr><td>PV 功率始终为 0</td><td>仿真时间在夜间（18:00~6:00），修改仿真起始时间</td></tr>
<tr><td>修改配置不生效</td><td>需先停止实例 → 修改 → 重新启动</td></tr>
<tr><td>IEC104 客户端无法连接</td><td>确认实例已启动，每个实例只接受一个客户端</td></tr>
<tr><td>测点值不变化</td><td>确认自动变化策略已启用，检查参数是否正确</td></tr>
</tbody></table>

<div class="info-box info"><div class="label">💡 版本</div>GridSim v3.2.1 | 2026-07-08</div>
</div>"""

# Assemble and write
final = f"<!DOCTYPE html>\n<html lang=\"zh-CN\">\n{head_html}\n<body>\n{sidebar}\n{body}\n</body>\n</html>"
with open(OUT_FILE, 'w', encoding='utf-8') as f:
    f.write(final)
print(f"Generated: {OUT_FILE}")
print(f"Size: {os.path.getsize(OUT_FILE) // 1024} KB")
