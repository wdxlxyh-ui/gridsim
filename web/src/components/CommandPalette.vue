<template>
  <teleport to="body">
    <transition name="cmd-fade">
      <div v-if="visible" class="cmd-overlay" @click.self="close" @keydown.esc="close">
        <div class="cmd-palette" ref="paletteRef">
          <div class="cmd-input-wrap">
            <span class="cmd-search-icon">⌕</span>
            <input
              ref="inputRef"
              v-model="query"
              class="cmd-input"
              placeholder="搜索页面或执行操作..."
              @keydown.down.prevent="nextItem"
              @keydown.up.prevent="prevItem"
              @keydown.enter="executeCurrent"
            />
            <kbd class="cmd-hint">ESC</kbd>
          </div>
          <div class="cmd-groups" v-if="filteredGroups.length > 0">
            <div v-for="group in filteredGroups" :key="group.label" class="cmd-group">
              <div class="cmd-group-label">{{ group.label }}</div>
              <div
                v-for="(item, idx) in group.items"
                :key="item.id"
                class="cmd-item"
                :class="{ active: globalIdx === groupStart(group) + idx }"
                @click="execute(item)"
                @mouseenter="globalIdx = groupStart(group) + idx"
              >
                <span class="cmd-item-icon">{{ item.icon }}</span>
                <span class="cmd-item-label">{{ item.label }}</span>
                <span class="cmd-item-desc">{{ item.desc }}</span>
                <span v-if="item.shortcut" class="cmd-item-shortcut">{{ item.shortcut }}</span>
              </div>
            </div>
          </div>
          <div v-else-if="query" class="cmd-empty">
            没有匹配的结果
          </div>
        </div>
      </div>
    </transition>
  </teleport>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'

interface CmdItem {
  id: string
  icon: string
  label: string
  desc: string
  shortcut?: string
  action: () => void
}

interface CmdGroup {
  label: string
  items: CmdItem[]
}

const router = useRouter()
const visible = ref(false)
const query = ref('')
const globalIdx = ref(0)
const inputRef = ref<HTMLInputElement>()
const paletteRef = ref<HTMLElement>()

const commands = computed<CmdGroup[]>(() => [
  {
    label: '页面导航',
    items: [
      { id: 'nav-dash', icon: '📊', label: '仪表盘', desc: '查看全局运行状态', action: () => router.push('/dashboard') },
      { id: 'nav-config', icon: '⚙️', label: '配置管理', desc: '管理实例', action: () => router.push('/config') },
      { id: 'nav-monitor', icon: '🖥️', label: '运行监控', desc: '实例运行状态', action: () => router.push('/monitor') },
      { id: 'nav-trend', icon: '📈', label: '实时趋势', desc: '测点趋势图表', action: () => router.push('/trend') },
      { id: 'nav-proxy', icon: '🔌', label: '接口测试', desc: 'HTTP API 测试工具', action: () => router.push('/proxy') },
    ],
  },
  {
    label: '操作',
    items: [
      { id: 'act-collapse', icon: '📐', label: '切换侧边栏', desc: '展开/收起导航栏', action: () => window.dispatchEvent(new CustomEvent('toggle-sidebar')) },
      { id: 'act-refresh', icon: '🔄', label: '刷新当前页', desc: '重新加载当前视图', action: () => window.location.reload() },
      { id: 'act-logout', icon: '🚪', label: '退出登录', desc: '返回登录页面', action: () => { localStorage.removeItem('iec104_token'); router.push('/login') } },
    ],
  },
])

const filteredGroups = computed(() => {
  if (!query.value) return commands.value
  const q = query.value.toLowerCase()
  return commands.value
    .map(g => ({
      label: g.label,
      items: g.items.filter(i =>
        i.label.toLowerCase().includes(q) ||
        i.desc.toLowerCase().includes(q) ||
        i.id.toLowerCase().includes(q)
      ),
    }))
    .filter(g => g.items.length > 0)
})

const flatItems = computed(() => {
  const all: CmdItem[] = []
  for (const g of filteredGroups.value) {
    all.push(...g.items)
  }
  return all
})

function groupStart(group: CmdGroup): number {
  let idx = 0
  for (const g of filteredGroups.value) {
    if (g === group) return idx
    idx += g.items.length
  }
  return 0
}

function nextItem() {
  if (flatItems.value.length === 0) return
  globalIdx.value = (globalIdx.value + 1) % flatItems.value.length
  scrollToActive()
}

function prevItem() {
  if (flatItems.value.length === 0) return
  globalIdx.value = (globalIdx.value - 1 + flatItems.value.length) % flatItems.value.length
  scrollToActive()
}

function scrollToActive() {
  nextTick(() => {
    const el = paletteRef.value?.querySelector('.cmd-item.active') as HTMLElement
    el?.scrollIntoView({ block: 'nearest' })
  })
}

function executeCurrent() {
  const item = flatItems.value[globalIdx.value]
  if (item) execute(item)
}

function execute(item: CmdItem) {
  close()
  setTimeout(() => item.action(), 150)
}

function close() {
  visible.value = false
  query.value = ''
  globalIdx.value = 0
}

function onKeydown(e: KeyboardEvent) {
  if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
    e.preventDefault()
    visible.value = !visible.value
    if (visible.value) {
      nextTick(() => inputRef.value?.focus())
    }
  }
  if (e.key === 'Escape' && visible.value) {
    close()
  }
}

watch(visible, (v) => {
  if (v) nextTick(() => inputRef.value?.focus())
})

onMounted(() => {
  document.addEventListener('keydown', onKeydown)
})

onUnmounted(() => {
  document.removeEventListener('keydown', onKeydown)
})
</script>

<style scoped>
.cmd-overlay {
  position: fixed;
  inset: 0;
  z-index: 9999;
  display: flex;
  justify-content: center;
  padding-top: 15vh;
  background: rgba(0, 0, 0, 0.5);
  backdrop-filter: blur(4px);
}
.cmd-palette {
  width: 560px;
  max-height: 400px;
  background: var(--bg-card, #141b2d);
  border: 1px solid var(--border-color, #2d3a4e);
  border-radius: 12px;
  box-shadow: 0 8px 40px rgba(0, 0, 0, 0.5);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.cmd-input-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 16px;
  border-bottom: 1px solid var(--border-color, #2d3a4e);
}
.cmd-search-icon {
  font-size: 18px;
  color: var(--text-muted, #64748b);
  flex-shrink: 0;
}
.cmd-input {
  flex: 1;
  border: none;
  background: transparent;
  color: var(--text-primary, #e2e8f0);
  font-size: 15px;
  outline: none;
  font-family: inherit;
}
.cmd-input::placeholder {
  color: var(--text-muted, #64748b);
}
.cmd-hint {
  font-size: 11px;
  padding: 2px 6px;
  border-radius: 4px;
  background: var(--bg-hover, #1e2736);
  color: var(--text-muted, #64748b);
  border: 1px solid var(--border-color, #2d3a4e);
}
.cmd-groups {
  flex: 1;
  overflow-y: auto;
  padding: 6px 0;
}
.cmd-group-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--text-muted, #64748b);
  padding: 8px 16px 4px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}
.cmd-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 16px;
  cursor: pointer;
  transition: background 0.1s;
}
.cmd-item:hover,
.cmd-item.active {
  background: var(--accent-light, rgba(59, 130, 246, 0.1));
}
.cmd-item-icon {
  font-size: 16px;
  width: 24px;
  text-align: center;
  flex-shrink: 0;
}
.cmd-item-label {
  font-size: 14px;
  color: var(--text-primary, #e2e8f0);
  flex-shrink: 0;
}
.cmd-item-desc {
  font-size: 12px;
  color: var(--text-muted, #64748b);
  margin-left: 4px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.cmd-item-shortcut {
  margin-left: auto;
  font-size: 11px;
  padding: 1px 6px;
  border-radius: 4px;
  background: var(--bg-hover, #1e2736);
  color: var(--text-muted, #64748b);
  flex-shrink: 0;
}
.cmd-empty {
  padding: 32px;
  text-align: center;
  color: var(--text-muted, #64748b);
  font-size: 13px;
}
.cmd-fade-enter-active,
.cmd-fade-leave-active {
  transition: opacity 0.15s ease;
}
.cmd-fade-enter-from,
.cmd-fade-leave-to {
  opacity: 0;
}
.cmd-fade-enter-active .cmd-palette {
  transition: transform 0.2s cubic-bezier(0.22, 1, 0.36, 1);
}
.cmd-fade-enter-from .cmd-palette {
  transform: translateY(-10px) scale(0.97);
}
</style>
