<template>
  <teleport to="body">
    <!-- Overlay mask with cutout highlight -->
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
          <!-- Highlight border ring -->
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
        <div
          v-if="currentStep"
          class="onboard-bubble"
          :style="bubbleStyle"
          @click.stop
        >
          <!-- Progress dots -->
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

          <!-- Action hint -->
          <div v-if="currentStep.action" class="onboard-action-hint">
            <span class="onboard-action-icon">👆</span> {{ currentStep.action }}
          </div>

          <div class="onboard-footer">
            <el-button size="small" text @click="skipAll" class="onboard-skip-btn">跳过引导</el-button>
            <div class="onboard-nav">
              <el-button v-if="stepIndex > 0" size="small" @click="prev">上一步</el-button>
              <el-button
                v-if="stepIndex < steps.length - 1"
                size="small"
                type="primary"
                @click="next"
              >
                下一步
              </el-button>
              <el-button v-else size="small" type="primary" @click="finish">
                完成 🎉
              </el-button>
            </div>
          </div>
        </div>
      </div>
    </transition>
  </teleport>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'

interface Step {
  title: string
  desc: string
  /** CSS selector for the element to highlight (optional) */
  selector?: string
  /** Bubble placement relative to highlight: top | bottom | left | right */
  placement?: 'top' | 'bottom' | 'left' | 'right'
  /** Action hint text */
  action?: string
}

const steps: Step[] = [
  {
    title: '欢迎使用 GridSim 👋',
    desc: '这里是仪表盘，展示所有实例的运行状态。让我们花 1 分钟快速了解核心功能。',
    placement: 'bottom',
  },
  {
    title: '📊 实例状态总览',
    desc: '这几张卡片展示了总实例数、运行中、已停止和在线客户端数量。点击卡片可快速筛选下方实例列表。',
    selector: '.stats-grid',
    placement: 'bottom',
  },
  {
    title: '⚙️ 配置管理',
    desc: '在配置管理页，你可以新建仿真实例、上传点表（Excel）、批量启停，以及编辑实例参数。',
    selector: '.nav-item[data-path="/config"], .app-sidebar .nav-item:nth-child(2)',
    placement: 'right',
  },
  {
    title: '🖥️ 运行监控',
    desc: '监控页展示所有实例的实时状态，包括连接数、告警和运行时长。点击实例可进入详情页操作测点。',
    selector: '.nav-item[data-path="/monitor"], .app-sidebar .nav-item:nth-child(3)',
    placement: 'right',
  },
  {
    title: '🔍 实例详情 — 置数 & 策略',
    desc: '在实例详情页，可以对单个测点手动置数、修改品质描述 QDS，或配置自动变化策略（递增、随机、CSV回放等）。',
    selector: '.instance-grid .instance-card:first-child',
    placement: 'bottom',
  },
  {
    title: '📈 实时趋势',
    desc: '在趋势页选择多个测点，实时查看其数值变化曲线，方便验证 SCADA 侧数据是否正确接收。',
    selector: '.nav-item[data-path="/trend"], .app-sidebar .nav-item:nth-child(4)',
    placement: 'right',
  },
  {
    title: '⚡ 快捷操作',
    desc: '这里汇集了最常用的快捷入口：新建实例、进入监控、查看趋势、接口测试，一键直达。',
    selector: '.quick-actions-card',
    placement: 'top',
  },
  {
    title: '🎉 引导完成！',
    desc: '你已了解 GridSim 的核心操作流程。建议从"配置管理"开始，上传点表、创建实例并启动，然后去"实例详情"操作测点。',
    placement: 'bottom',
    action: '点击右下角 ❓ 按钮可随时重新查看本引导',
  },
]

const active = ref(false)
const stepIndex = ref(0)
const highlightRect = ref<{ x: number; y: number; width: number; height: number } | null>(null)
const viewW = ref(window.innerWidth)
const viewH = ref(window.innerHeight)

const currentStep = computed(() => steps[stepIndex.value] ?? null)

/** Bubble positioning */
const bubbleStyle = computed(() => {
  const BUBBLE_W = 340
  const BUBBLE_H = 220
  const MARGIN = 16

  if (!highlightRect.value || !currentStep.value?.selector) {
    // center of screen
    return {
      position: 'fixed' as const,
      left: `${(viewW.value - BUBBLE_W) / 2}px`,
      top: `${(viewH.value - BUBBLE_H) / 2}px`,
      width: `${BUBBLE_W}px`,
    }
  }

  const r = highlightRect.value
  const placement = currentStep.value?.placement ?? 'bottom'
  let left = 0
  let top = 0

  if (placement === 'bottom') {
    left = r.x + r.width / 2 - BUBBLE_W / 2
    top = r.y + r.height + 16 + 8
  } else if (placement === 'top') {
    left = r.x + r.width / 2 - BUBBLE_W / 2
    top = r.y - BUBBLE_H - 16 - 8
  } else if (placement === 'right') {
    left = r.x + r.width + 16 + 8
    top = r.y + r.height / 2 - BUBBLE_H / 2
  } else if (placement === 'left') {
    left = r.x - BUBBLE_W - 16 - 8
    top = r.y + r.height / 2 - BUBBLE_H / 2
  }

  // Clamp inside viewport
  left = Math.max(MARGIN, Math.min(left, viewW.value - BUBBLE_W - MARGIN))
  top = Math.max(MARGIN, Math.min(top, viewH.value - BUBBLE_H - MARGIN))

  return {
    position: 'fixed' as const,
    left: `${left}px`,
    top: `${top}px`,
    width: `${BUBBLE_W}px`,
  }
})

function updateHighlight() {
  const step = currentStep.value
  if (!step?.selector) {
    highlightRect.value = null
    return
  }
  // Try multiple selectors (comma-split)
  const selectors = step.selector.split(',').map(s => s.trim())
  let el: Element | null = null
  for (const sel of selectors) {
    el = document.querySelector(sel)
    if (el) break
  }
  if (!el) {
    highlightRect.value = null
    return
  }
  const rect = el.getBoundingClientRect()
  highlightRect.value = { x: rect.left, y: rect.top, width: rect.width, height: rect.height }
}

function onResize() {
  viewW.value = window.innerWidth
  viewH.value = window.innerHeight
  updateHighlight()
}

watch(stepIndex, () => {
  nextTick(updateHighlight)
})

watch(active, (v) => {
  if (v) {
    stepIndex.value = 0
    nextTick(updateHighlight)
  }
})

/** Public API */
function start() {
  active.value = true
}

function next() {
  if (stepIndex.value < steps.length - 1) stepIndex.value++
}

function prev() {
  if (stepIndex.value > 0) stepIndex.value--
}

function skipAll() {
  active.value = false
}

function finish() {
  active.value = false
}

function handleOverlayClick() {
  // clicking the dark area advances to next step (or closes on last)
  if (stepIndex.value < steps.length - 1) {
    next()
  } else {
    finish()
  }
}

onMounted(() => {
  window.addEventListener('resize', onResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', onResize)
})

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

@keyframes ring-dash {
  to { stroke-dashoffset: -18; }
}

/* Bubble */
.onboard-bubble {
  background: var(--bg-card, #141b2d);
  border: 1px solid rgba(59, 130, 246, 0.4);
  border-radius: 14px;
  padding: 20px;
  box-shadow: 0 12px 48px rgba(0, 0, 0, 0.6), 0 0 0 1px rgba(59,130,246,0.1);
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

.onboard-progress {
  display: flex;
  gap: 5px;
  align-items: center;
}

.onboard-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--border-color, #334155);
  transition: all 0.25s;
}

.onboard-dot.done {
  background: rgba(59, 130, 246, 0.5);
}

.onboard-dot.active {
  width: 18px;
  border-radius: 3px;
  background: #3b82f6;
}

.onboard-step-tag {
  font-size: 11px;
  color: #3b82f6;
  font-weight: 600;
  letter-spacing: 0.5px;
  text-transform: uppercase;
}

.onboard-title {
  font-size: 15px;
  font-weight: 700;
  color: var(--text-primary, #e2e8f0);
  line-height: 1.3;
}

.onboard-desc {
  font-size: 13px;
  color: var(--text-secondary, #94a3b8);
  line-height: 1.6;
}

.onboard-action-hint {
  font-size: 12px;
  color: #f59e0b;
  background: rgba(245, 158, 11, 0.08);
  border: 1px solid rgba(245, 158, 11, 0.2);
  border-radius: 6px;
  padding: 6px 10px;
  display: flex;
  align-items: center;
  gap: 6px;
}

.onboard-action-icon {
  font-size: 14px;
}

.onboard-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 4px;
}

.onboard-skip-btn {
  color: var(--text-muted, #64748b) !important;
  font-size: 12px !important;
  padding: 0 !important;
}

.onboard-skip-btn:hover {
  color: var(--text-secondary, #94a3b8) !important;
}

.onboard-nav {
  display: flex;
  gap: 8px;
}

/* Transitions */
.onboard-fade-enter-active,
.onboard-fade-leave-active {
  transition: opacity 0.2s ease;
}

.onboard-fade-enter-from,
.onboard-fade-leave-to {
  opacity: 0;
}
</style>
