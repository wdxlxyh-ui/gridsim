<template>
  <div class="collection-panel">
    <div class="collection-header">
      <el-input v-model="searchText" placeholder="搜索请求..." size="small" clearable />
      <div class="collection-actions">
        <el-button size="small" @click="$emit('add-folder')">📁 文件夹</el-button>
        <el-button size="small" @click="$emit('add-request')">＋ 请求</el-button>
      </div>
    </div>
    <div class="collection-tree">
      <div v-for="folder in filteredCollections" :key="folder.id" class="tree-folder"
        @dragover.prevent="onDragOver($event, folder)"
        @dragleave="onDragLeave($event, folder)"
        @drop="onDrop($event, folder)">
        <div class="tree-folder-header" :class="{ selected: folder.id === activeFolderId }"
          @click="$emit('select-folder', folder)">
          <span class="arrow" :class="{ open: folder._open }">▶</span>
          <span class="folder-icon">📁</span>
          <span class="folder-name">{{ folder.name }}</span>
          <span class="folder-actions">
            <el-button text size="small" @click.stop="$emit('rename-folder', folder)">✏</el-button>
            <el-button text size="small" @click.stop="$emit('delete-folder', folder.id)">🗑</el-button>
          </span>
        </div>
        <div class="tree-children" :class="{ collapsed: !folder._open }">
          <div v-for="req in (folder.children || [])" :key="req.id"
            class="tree-request" :class="{ active: req.id === activeRequestId }"
            draggable="true"
            @dragstart="onDragStart($event, req, folder)"
            @dragend="onDragEnd"
            @click="$emit('select-request', req)">
            <span class="req-method" :class="req.method">{{ req.method }}</span>
            <span class="req-name">{{ req.name }}</span>
            <span class="req-actions">
              <el-button text size="small" @click.stop="$emit('copy-request', req)">📋</el-button>
              <el-button text size="small" @click.stop="$emit('delete-request', req.id)">🗑</el-button>
            </span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import type { CollectionItem } from '../../api'

const props = defineProps<{
  collections: (CollectionItem & { _open?: boolean })[]
  activeRequestId: string
  activeFolderId: string
}>()

defineEmits<{
  'add-folder': []
  'add-request': []
  'select-folder': [folder: any]
  'select-request': [req: CollectionItem]
  'rename-folder': [folder: CollectionItem]
  'delete-folder': [id: string]
  'copy-request': [req: CollectionItem]
  'delete-request': [id: string]
  'move-request': [reqId: string, fromFolder: any, toFolder: any]
}>()

const searchText = ref('')
let dragReq: CollectionItem | null = null
let dragSrcFolder: any = null

const filteredCollections = computed(() => {
  if (!searchText.value) return props.collections
  const q = searchText.value.toLowerCase()
  return props.collections.filter(f => f.name.toLowerCase().includes(q) || (f.children || []).some(r => r.name.toLowerCase().includes(q)))
})

function onDragStart(e: DragEvent, req: CollectionItem, folder: any) {
  dragReq = req; dragSrcFolder = folder
  e.dataTransfer!.effectAllowed = 'move'
  e.dataTransfer!.setData('text/plain', req.id);
  (e.target as HTMLElement).classList.add('dragging')
}

function onDragEnd(e: DragEvent) {
  ;(e.target as HTMLElement).classList.remove('dragging')
  dragReq = null; dragSrcFolder = null
  document.querySelectorAll('.tree-folder.drag-over').forEach(el => el.classList.remove('drag-over'))
}

function onDragOver(e: DragEvent, folder: any) {
  if (!dragReq) return
  if (folder.id === dragSrcFolder?.id) return
  e.preventDefault()
  e.dataTransfer!.dropEffect = 'move';
  (e.currentTarget as HTMLElement).classList.add('drag-over')
}

function onDragLeave(e: DragEvent, _folder: any) {
  ;(e.currentTarget as HTMLElement).classList.remove('drag-over')
}

function onDrop(e: DragEvent, targetFolder: any) {
  ;(e.currentTarget as HTMLElement).classList.remove('drag-over')
  if (!dragReq || !dragSrcFolder) return
  if (targetFolder.id === dragSrcFolder.id) return
}
</script>

<style scoped>
.collection-panel { width: 260px; background: #111827; border-right: 1px solid #1e293b; display: flex; flex-direction: column; }
.collection-header { padding: 12px; border-bottom: 1px solid #1e293b; display: flex; flex-direction: column; gap: 8px; }
.collection-actions { display: flex; gap: 6px; }
.collection-tree { flex: 1; overflow-y: auto; padding: 8px 0; }
.tree-folder-header { display: flex; align-items: center; gap: 6px; padding: 6px 12px; cursor: pointer; font-size: 13px; color: #94a3b8; }
.tree-folder-header:hover { background: #1a1f2e; color: #e2e8f0; }
.tree-folder-header.selected { background: rgba(245,158,11,0.06); color: #f59e0b; }
.arrow { font-size: 10px; transition: transform 0.2s; width: 12px; text-align: center; }
.arrow.open { transform: rotate(90deg); }
.tree-children { padding-left: 16px; }
.tree-children.collapsed { display: none; }
.tree-request { display: flex; align-items: center; gap: 6px; padding: 5px 12px 5px 28px; cursor: pointer; font-size: 12px; color: #64748b; border-left: 2px solid transparent; }
.tree-request:hover { background: #1a1f2e; color: #e2e8f0; }
.tree-request.active { color: #f59e0b; background: rgba(245,158,11,0.08); border-left-color: #f59e0b; }
.req-method { font-family: 'JetBrains Mono', monospace; font-size: 10px; font-weight: 600; padding: 1px 4px; border-radius: 3px; }
.req-method.GET { background: rgba(34,197,94,0.15); color: #22c55e; }
.req-method.POST { background: rgba(245,158,11,0.15); color: #f59e0b; }
.req-method.PUT { background: rgba(59,130,246,0.15); color: #3b82f6; }
.req-method.DELETE { background: rgba(239,68,68,0.15); color: #ef4444; }
.req-actions { display: none; gap: 2px; }
.tree-request:hover .req-actions { display: flex; }
.tree-request.dragging { opacity: 0.4; }
.tree-folder.drag-over { background: rgba(245,158,11,0.08); border-radius: 4px; }
.tree-folder.drag-over .tree-folder-header { color: #f59e0b; }
</style>
