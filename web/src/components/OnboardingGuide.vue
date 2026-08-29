<template>
  <teleport to="body">
    <transition name="onboard-fade">
      <div v-if="active" class="onboard-overlay" @click.self="handleOverlayClick">
        <!-- SVG cutout mask -->
        <svg class="onboard-mask" :width="viewW" :height="viewH" @click.self="handleOverlayClick">
          <defs>
            <mask id="onboard-cutout">
              <rect width="100%" height="100%" fill="white" />
              <rect
                v-if="highlightRect"
                :x="highlightRect.x - 8"
                :y="highlightRect.y - 8"
                :width="highlightRect.width + 16"
                :height="highlightRect.height + 16"
                rx="8"
                fill="black"
              />
            </mask>
          </defs>
          <rect width="100%" height="100%" fill="rgba(0,0,0,0.55)" mask="url(#onboard-cutout)" />
          <rect
            v-if="highlightRect"
            :x="highlightRect.x - 8"
            :y="highlightRect.y - 8"
            :width="highlightRect.width + 16"
            :height="highlightRect.height + 16"
            rx="8"
            fill="none"
            stroke="#3b82f6"
            stroke-width="2"
            stroke-dasharray="6 3"
            class="onboard-ring"
          />
        </svg>

        <!-- Tooltip bubble -->
        <div v-if="currentStep" class="onboard-bubble" :style="bubbleStyle" @click.stop>
          <div class="onboard-progress">
            <span
              v-for="(_, i) in steps"
              :key="i"
              class="onboard-dot"
              :class="{ active: i === stepIndex, done: i < stepIndex }"
            />
          </div>
          <div class="onboard-step-tag">步骤 {{ stepIndex + 1 }} / {{ steps.length }}</div>
          <div class="onboard-title">{{ currentStep.title }}</div>
          <div class="onboard-desc">{{ currentStep.desc }}</div>
          <div v-if="currentStep.action" class="onboard-action-hint">
            <span class="onboard-action-icon">👆</span> {{ currentStep.action }}
          </div>
          <div class="onboard-footer">
            <el-button size="small" text @click="skipAll" class="onboard-skip-btn">跳过引导</el-button>
            <div class="onboard-nav">
              <el-button v-if="stepIndex > 0" size="small" @click="prev">上一步</el-button>
              <el-button
                v-if="stepIndex < steps.length - 1"
                size="small" type="primary" @click="next"
              >下一步</el-button>
              <el-button v-else size="small" type="primary" @click="finish">完成 🎉</el-button>
            </div>
          </div>
        </div>
      </div>
    </transition>
  </teleport>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'

interface Step {
  title: string
  desc: string
  selector?: string
  placement?: 'top' | 'bottom' | 'left' | 'right'
  action?: string
  /** Route to navigate to before highlighting */
  route?: string
}

const basicSteps: Step[] = [
  {
    title: '欢迎使用 GridSim 👋',
    desc: '这里是仪表盘，展示所有实例的运行状态。让我们花 1 分钟快速了解核心功能。',
    route: '/dashboard',
    placement: 'bottom',
  },
  {
    title: '📊 实例状态总览',
    desc: '这几张卡片展示了总实例数、运行中、已停止和在线客户端数量。点击卡片可快速筛选下方实例列表。',
    route: '/dashboard',
    selector: '.stats-grid',
    placement: 'bottom',
  },
  {
    title: '⚙️ 配置管理',
    desc: '在配置管理页，你可以新建仿真实例、上传点表（Excel）、批量启停，以及编辑实例参数。',
    route: '/config',
    selector: '.el-card',
    placement: 'bottom',
  },
  {
    title: '🖥️ 运行监控',
    desc: '监控页展示所有实例的实时状态，包括连接数、告警和运行时长。点击实例可进入详情页操作测点。',
    route: '/monitor',
    selector: '.el-card',
    placement: 'bottom',
  },
  {
    title: '🔍 实例详情 — 置数 & 策略',
    desc: '在实例详情页，可以对单个测点手动置数、修改品质描述 QDS，或配置自动变化策略（递增、随机、CSV回放等）。',
    route: '/monitor',
    selector: '.el-card',
    placement: 'bottom',
  },
  {
    title: '📈 实时趋势',
    desc: '在趋势页选择多个测点，实时查看其数值变化曲线，方便验证 SCADA 侧数据是否正确接收。',
    route: '/trend',
    selector: '.el-card',
    placement: 'bottom',
  },
  {
    title: '⚡ 快捷操作',
    desc: '回到仪表盘，这里汇集了最常用的快捷入口：新建实例、进入监控、查看趋势、接口测试，一键直达。',
    route: '/dashboard',
    selector: '.quick-actions-card',
    placement: 'top',
  },
  {
    title: '🎉 引导完成！',
    desc: '你已了解 GridSim 的核心操作流程。建议从"配置管理"开始，上传点表、创建实例并启动，然后去"实例详情"操作测点。',
    route: '/dashboard',
    placement: 'bottom',
    action: '点击右下角 ❓ 按钮可随时重新查看本引导',
  },
]

const advancedSteps: Step[] = [
  {
    title: '⚙️ 配置管理入口',
    desc: '配置管理是管理所有仿真实例的核心页面。在这里可以新建、编辑、删除实例，以及批量启停。',
    route: '/config',
    selector: '.el-card',
    placement: 'bottom',
  },
  {
    title: '➕ 新建实例',
    desc: '点击右上角"添加实例"按钮，打开新建向导。向导分4步：选规约类型 → 填基本参数 → 上传点表 → 确认创建。',
    route: '/config',
    selector: '.el-button--primary',
    placement: 'bottom',
    action: '点击"添加实例"按钮开始创建',
  },
  {
    title: '📋 实例列表',
    desc: '创建完成后，实例出现在这个列表中。可以单独或批量启动/停止/删除实例，勾选多行后会出现批量操作栏。',
    route: '/config',
    selector: '.el-table',
    placement: 'top',
  },
  {
    title: '🖥️ 运行监控',
    desc: '实例启动后，在监控页可以看到实时状态：连接的客户端数、运行时长、告警信息。点击实例名称进入详情页。',
    route: '/monitor',
    selector: '.el-card',
    placement: 'bottom',
  },
  {
    title: '📍 手动置数',
    desc: '在实例详情页，测点列表的"置数"列可以直接输入数值并立即生效。AI（遥测）和 DI（遥信）均支持手动置数。',
    route: '/monitor',
    selector: '.el-card',
    placement: 'bottom',
    action: '点击监控页任意实例名称进入详情页',
  },
  {
    title: '🤖 自动变化策略',
    desc: '点击测点行右侧的策略按钮，可配置4种自动变化策略：递增/递减（线性变化）、随机波动（区间随机）、正弦波（周期振荡）、CSV回放（按文件序列播放）。',
    route: '/monitor',
    selector: '.el-card',
    placement: 'bottom',
    action: '进入实例详情页后，点击测点行右侧的"策略"按钮',
  },
  {
    title: '📈 实时趋势验证',
    desc: '配置策略后，可以在趋势页选择对应测点，观察数值变化曲线，验证 SCADA 侧是否正确接收数据。',
    route: '/trend',
    selector: '.el-card',
    placement: 'bottom',
  },
  {
    title: '🎉 高级引导完成！',
    desc: '你已掌握 GridSim 的完整操作流程。如有疑问，可随时重新查看引导，或参考页面内的提示说明。',
    route: '/dashboard',
    placement: 'bottom',
    action: '点击右下角 ❓ 按钮可随时重新查看引导',
  },
]

const router = useRouter()
const active = ref(false)
const stepIndex = ref(0)
const steps = ref<Step[]>(basicSteps)
const highlightRect = ref<{ x: number; y: number; width: number; height: number } | null>(null)
const viewW = ref(window.innerWidth)
const viewH = ref(window.innerHeight)

const currentStep = computed(() => steps.value[stepIndex.value] ?? null)

const bubbleStyle = computed(() => {
  const BUBBLE_W = 340
  const BUBBLE_H = 220
  const MARGIN = 16

  if (!highlightRect.value || !currentStep.value?.selector) {
    return {
      position: 'fixed' as const,
      left: `${(viewW.value - BUBBLE_W) / 2}px`,
      top: `${(viewH.value - BUBBLE_H) / 2}px`,
      width: `${BUBBLE_W}px`,
    }
  }

  const r = highlightRect.value
  const placement = currentStep.value?.placement ?? 'bottom'
  let left = 0, top = 0

  if (placement === 'bottom') { left = r.x + r.width / 2 - BUBBLE_W / 2; top = r.y + r.height + 24 }
  else if (placement === 'top') { left = r.x + r.width / 2 - BUBBLE_W / 2; top = r.y - BUBBLE_H - 24 }
  else if (placement === 'right') { left = r.x + r.width + 24; top = r.y + r.height / 2 - BUBBLE_H / 2 }
  else if (placement === 'left') { left = r.x - BUBBLE_W - 24; top = r.y + r.height / 2 - BUBBLE_H / 2 }

  left = Math.max(MARGIN, Math.min(left, viewW.value - BUBBLE_W - MARGIN))
  top = Math.max(MARGIN, Math.min(top, viewH.value - BUBBLE_H - MARGIN))

  return { position: 'fixed' as const, left: `${left}px`, top: `${top}px`, width: `${BUBBLE_W}px` }
})

async function navigateAndHighlight(step: Step) {
  if (step.route) {
    const currentPath = router.currentRoute.value.path
    if (currentPath !== step.route) {
      await router.push(step.route)
      // wait for page transition + render
      await new Promise(r => setTimeout(r, 400))
    }
  }
  await nextTick()
  updateHighlight()
}

function updateHighlight() {
  const step = currentStep.value
  if (!step?.selector) { highlightRect.value = null; return }
  const selectors = step.selector.split(',').map(s => s.trim())
  let el: Element | null = null
  for (const sel of selectors) { el = document.querySelector(sel); if (el) break }
  if (!el) { highlightRect.value = null; return }
  const rect = el.getBoundingClientRect()
  highlightRect.value = { x: rect.left, y: rect.top, width: rect.width, height: rect.height }
}

function onResize() {
  viewW.value = window.innerWidth
  viewH.value = window.innerHeight
  updateHighlight()
}

async function next() {
  if (stepIndex.value < steps.value.length - 1) {
    stepIndex.value++
    await navigateAndHighlight(steps.value[stepIndex.value])
  }
}

async function prev() {
  if (stepIndex.value > 0) {
    stepIndex.value--
    await navigateAndHighlight(steps.value[stepIndex.value])
  }
}

function skipAll() { active.value = false }
function finish() { active.value = false }

function handleOverlayClick() {
  if (stepIndex.value < steps.value.length - 1) next()
  else finish()
}

async function start(mode: 'basic' | 'advanced' = 'basic') {
  steps.value = mode === 'advanced' ? advancedSteps : basicSteps
  stepIndex.value = 0
  active.value = true
  await navigateAndHighlight(steps.value[0])
}

onMounted(() => window.addEventListener('resize', onResize))
onUnmounted(() => window.removeEventListener('resize', onResize))

defineExpose({ start })
</script>

<style scoped>
.onboard-overlay {
  position: fixed;
  inset: 0;
  z-index: 8000;
  pointer-events: all;
}
.onboard-mask {
  position: absolute;
  inset: 0;
  display: block;
}
.onboard-ring {
  animation: ring-dash 1.5s linear infinite;
}
@keyframes ring-dash { to { stroke-dashoffset: -18; } }

.onboard-bubble {
  background: var(--bg-card, #141b2d);
  border: 1px solid rgba(59, 130, 246, 0.4);
  border-radius: 14px;
  padding: 20px;
  box-shadow: 0 12px 48px rgba(0,0,0,0.6), 0 0 0 1px rgba(59,130,246,0.1);
  display: flex;
  flex-direction: column;
  gap: 10px;
  z-index: 8001;
  animation: bubble-pop 0.25s cubic-bezier(0.34, 1.56, 0.64, 1);
}
@keyframes bubble-pop {
  from { transform: scale(0.92); opacity: 0; }
  to   { transform: scale(1); opacity: 1; }
}
.onboard-progress { display: flex; gap: 5px; align-items: center; }
.onboard-dot {
  width: 6px; height: 6px; border-radius: 50%;
  background: var(--border-color, #334155);
  transition: all 0.25s;
}
.onboard-dot.done { background: rgba(59,130,246,0.5); }
.onboard-dot.active { width: 18px; border-radius: 3px; background: #3b82f6; }
.onboard-step-tag { font-size: 11px; color: #3b82f6; font-weight: 600; letter-spacing: 0.5px; text-transform: uppercase; }
.onboard-title { font-size: 15px; font-weight: 700; color: var(--text-primary, #e2e8f0); line-height: 1.3; }
.onboard-desc { font-size: 13px; color: var(--text-secondary, #94a3b8); line-height: 1.6; }
.onboard-action-hint {
  font-size: 12px; color: #f59e0b;
  background: rgba(245,158,11,0.08);
  border: 1px solid rgba(245,158,11,0.2);
  border-radius: 6px; padding: 6px 10px;
  display: flex; align-items: center; gap: 6px;
}
.onboard-action-icon { font-size: 14px; }
.onboard-footer { display: flex; justify-content: space-between; align-items: center; margin-top: 4px; }
.onboard-skip-btn { color: var(--text-muted, #64748b) !important; font-size: 12px !important; padding: 0 !important; }
.onboard-skip-btn:hover { color: var(--text-secondary, #94a3b8) !important; }
.onboard-nav { display: flex; gap: 8px; }
.onboard-fade-enter-active, .onboard-fade-leave-active { transition: opacity 0.2s ease; }
.onboard-fade-enter-from, .onboard-fade-leave-to { opacity: 0; }
</style>
