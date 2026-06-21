<template>
  <div class="skeleton-screen" :class="`sk-${type}`">
    <!-- Dashboard -->
    <template v-if="type === 'dashboard'">
      <div class="sk-header">
        <div class="sk-block sk-h3 sk-w40"></div>
        <div class="sk-block sk-h4 sk-w20"></div>
      </div>
      <div class="sk-stats-grid">
        <div v-for="i in 6" :key="i" class="sk-card">
          <div class="sk-block sk-icon-round"></div>
          <div class="sk-block sk-h2 sk-w30" style="margin-top:8px"></div>
          <div class="sk-block sk-h4 sk-w50" style="margin-top:4px"></div>
          <div class="sk-block sk-h5 sk-w40" style="margin-top:8px"></div>
        </div>
      </div>
      <div class="sk-content-grid">
        <div class="sk-sidebar-card">
          <div class="sk-block sk-h3 sk-w40"></div>
          <div class="sk-divider"></div>
          <div v-for="i in 3" :key="i" class="sk-row">
            <div class="sk-block sk-dot"></div>
            <div class="sk-block sk-h4 sk-w30"></div>
            <div class="sk-block sk-h4 sk-w15" style="margin-left:auto"></div>
          </div>
        </div>
        <div class="sk-main-card">
          <div class="sk-block sk-h3 sk-w30"></div>
          <div class="sk-divider"></div>
          <div class="sk-grid-3col">
            <div v-for="i in 6" :key="i" class="sk-card sk-card-sm">
              <div class="sk-block sk-h4 sk-w60"></div>
              <div class="sk-block sk-h5 sk-w40" style="margin-top:6px"></div>
              <div class="sk-block sk-h5 sk-w30" style="margin-top:6px"></div>
            </div>
          </div>
        </div>
      </div>
      <div class="sk-card" style="margin-top:16px">
        <div class="sk-block sk-h3 sk-w25"></div>
        <div class="sk-divider"></div>
        <div class="sk-actions-grid">
          <div v-for="i in 4" :key="i" class="sk-action-item">
            <div class="sk-block sk-icon-rect"></div>
            <div>
              <div class="sk-block sk-h4 sk-w35"></div>
              <div class="sk-block sk-h5 sk-w55" style="margin-top:4px"></div>
            </div>
          </div>
        </div>
      </div>
    </template>

    <!-- Config -->
    <template v-else-if="type === 'config'">
      <div class="sk-header">
        <div class="sk-block sk-h3 sk-w30"></div>
        <div class="sk-block sk-btn"></div>
      </div>
      <div class="sk-card" style="margin-top:16px">
        <div v-for="i in 5" :key="i" class="sk-table-row">
          <div class="sk-block sk-h4 sk-w15"></div>
          <div class="sk-block sk-h4 sk-w20"></div>
          <div class="sk-block sk-h4 sk-w12"></div>
          <div class="sk-block sk-h4 sk-w10"></div>
          <div class="sk-block sk-h4 sk-w8"></div>
          <div class="sk-block sk-h4 sk-w15" style="margin-left:auto"></div>
        </div>
      </div>
    </template>

    <!-- Monitor -->
    <template v-else-if="type === 'monitor'">
      <div class="sk-header">
        <div class="sk-block sk-h3 sk-w25"></div>
        <div class="sk-block sk-btn"></div>
      </div>
      <div class="sk-grid-3col" style="margin-top:16px">
        <div v-for="i in 9" :key="i" class="sk-card sk-card-md">
          <div class="sk-block sk-h4 sk-w50"></div>
          <div class="sk-block sk-h3 sk-w30" style="margin-top:8px"></div>
          <div class="sk-block sk-h5 sk-w40" style="margin-top:6px"></div>
          <div class="sk-flex-row" style="margin-top:10px">
            <div class="sk-block sk-btn-sm"></div>
            <div class="sk-block sk-btn-sm"></div>
          </div>
        </div>
      </div>
    </template>

    <!-- Detail -->
    <template v-else-if="type === 'detail'">
      <div class="sk-header">
        <div class="sk-block sk-h3 sk-w35"></div>
        <div class="sk-block sk-btn"></div>
      </div>
      <div class="sk-flex-row" style="gap:12px;margin-top:16px">
        <div v-for="i in 4" :key="i" class="sk-card" style="flex:1;padding:12px">
          <div class="sk-block sk-h4 sk-w40"></div>
          <div class="sk-block sk-h2 sk-w25" style="margin-top:6px"></div>
        </div>
      </div>
      <div class="sk-card" style="margin-top:16px">
        <div class="sk-block sk-h3 sk-w20"></div>
        <div class="sk-divider"></div>
        <div v-for="i in 8" :key="i" class="sk-table-row">
          <div class="sk-block sk-h4 sk-w10"></div>
          <div class="sk-block sk-h4 sk-w25"></div>
          <div class="sk-block sk-h4 sk-w15"></div>
          <div class="sk-block sk-h4 sk-w12"></div>
          <div class="sk-block sk-h4 sk-w8"></div>
          <div class="sk-block sk-btn-sm" style="margin-left:auto"></div>
        </div>
      </div>
    </template>

    <!-- Trend -->
    <template v-else-if="type === 'trend'">
      <div class="sk-header">
        <div class="sk-block sk-h3 sk-w28"></div>
      </div>
      <div class="sk-flex-row" style="gap:8px;margin-top:12px">
        <div v-for="i in 5" :key="i" class="sk-block sk-btn-sm"></div>
      </div>
      <div class="sk-card" style="margin-top:12px;height:400px;display:flex;align-items:center;justify-content:center">
        <div class="sk-block" style="width:80%;height:60%;border-radius:8px"></div>
      </div>
    </template>

    <!-- Proxy -->
    <template v-else-if="type === 'proxy'">
      <div class="sk-flex-row" style="gap:12px;height:calc(100vh - 120px)">
        <div class="sk-card" style="width:260px;flex-shrink:0">
          <div class="sk-block sk-h3 sk-w40"></div>
          <div class="sk-divider"></div>
          <div v-for="i in 5" :key="i" class="sk-row">
            <div class="sk-block sk-icon-folder"></div>
            <div class="sk-block sk-h4 sk-w60"></div>
          </div>
        </div>
        <div class="sk-card" style="flex:1">
          <div class="sk-block sk-h3 sk-w35"></div>
          <div class="sk-divider"></div>
          <div class="sk-block" style="height:200px;border-radius:6px;margin-top:8px"></div>
          <div class="sk-block sk-h4 sk-w50" style="margin-top:12px"></div>
          <div class="sk-block" style="height:120px;border-radius:6px;margin-top:8px"></div>
        </div>
      </div>
    </template>

    <!-- Microgrid -->
    <template v-else-if="type === 'microgrid'">
      <div class="sk-flex-row" style="gap:12px;height:calc(100vh - 120px)">
        <div class="sk-card" style="width:220px;flex-shrink:0">
          <div class="sk-block sk-h3 sk-w40"></div>
          <div class="sk-divider"></div>
          <div v-for="i in 4" :key="i" class="sk-row">
            <div class="sk-block sk-icon-rect"></div>
            <div class="sk-block sk-h4 sk-w50"></div>
          </div>
        </div>
        <div class="sk-card" style="flex:1">
          <div class="sk-block" style="height:100%;border-radius:6px"></div>
        </div>
        <div class="sk-card" style="width:280px;flex-shrink:0">
          <div class="sk-block sk-h3 sk-w45"></div>
          <div class="sk-divider"></div>
          <div v-for="i in 5" :key="i" class="sk-row">
            <div class="sk-block sk-h4 sk-w50"></div>
            <div class="sk-block sk-h4 sk-w25" style="margin-left:auto"></div>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
withDefaults(defineProps<{
  type?: 'dashboard' | 'config' | 'monitor' | 'detail' | 'trend' | 'proxy' | 'microgrid'
}>(), {
  type: 'dashboard'
})
</script>

<style scoped>
/* ─── Shimmer Animation ─── */
@keyframes sk-shimmer {
  0% { background-position: -400px 0; }
  100% { background-position: 400px 0; }
}

.skeleton-screen {
  width: 100%;
}

/* ─── Base Block ─── */
.sk-block {
  background: linear-gradient(
    90deg,
    var(--bg-hover) 0%,
    rgba(59, 130, 246, 0.06) 40%,
    var(--bg-hover) 80%
  );
  background-size: 400px 100%;
  animation: sk-shimmer 1.8s ease-in-out infinite;
  border-radius: var(--radius-sm, 6px);
  height: 14px;
}

/* ─── Size Variants ─── */
.sk-h2 { height: 24px; border-radius: var(--radius-md, 8px); }
.sk-h3 { height: 18px; }
.sk-h4 { height: 13px; }
.sk-h5 { height: 10px; }

.sk-w8 { width: 8%; }
.sk-w10 { width: 10%; }
.sk-w12 { width: 12%; }
.sk-w15 { width: 15%; }
.sk-w20 { width: 20%; }
.sk-w25 { width: 25%; }
.sk-w28 { width: 28%; }
.sk-w30 { width: 30%; }
.sk-w35 { width: 35%; }
.sk-w40 { width: 40%; }
.sk-w45 { width: 45%; }
.sk-w50 { width: 50%; }
.sk-w55 { width: 55%; }
.sk-w60 { width: 60%; }

/* ─── Icon Blocks ─── */
.sk-icon-round {
  width: 36px;
  height: 36px;
  border-radius: 10px;
}

.sk-icon-rect {
  width: 42px;
  height: 42px;
  border-radius: 10px;
}

.sk-icon-folder {
  width: 18px;
  height: 18px;
  border-radius: 4px;
}

.sk-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

/* ─── Buttons ─── */
.sk-btn {
  width: 80px;
  height: 32px;
  border-radius: var(--radius-sm, 6px);
}

.sk-btn-sm {
  width: 56px;
  height: 28px;
  border-radius: var(--radius-sm, 6px);
}

/* ─── Layout Helpers ─── */
.sk-flex-row {
  display: flex;
  align-items: center;
}

.sk-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.sk-divider {
  height: 1px;
  background: var(--border-color);
  margin: 12px 0;
}

/* ─── Card ─── */
.sk-card {
  background: var(--el-bg-color-overlay, #141b2d);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg, 12px);
  padding: 16px;
}

.sk-card-sm { padding: 12px; }

.sk-card-md { padding: 14px; }

/* ─── Dashboard ─── */
.sk-stats-grid {
  display: grid;
  grid-template-columns: repeat(6, 1fr);
  gap: 12px;
  margin-bottom: 16px;
}

.sk-content-grid {
  display: grid;
  grid-template-columns: 260px 1fr;
  gap: 16px;
  margin-bottom: 16px;
}

.sk-sidebar-card {
  background: var(--el-bg-color-overlay, #141b2d);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg, 12px);
  padding: 16px;
}

.sk-main-card {
  background: var(--el-bg-color-overlay, #141b2d);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg, 12px);
  padding: 16px;
}

.sk-grid-3col {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 10px;
}

.sk-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 0;
}

.sk-table-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 0;
  border-bottom: 1px solid var(--border-color);
}

.sk-table-row:last-child { border-bottom: none; }

.sk-actions-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 10px;
}

.sk-action-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 16px;
  border: 1px solid var(--border-color);
  border-radius: 10px;
}

/* ─── Responsive ─── */
@media (max-width: 1200px) {
  .sk-stats-grid { grid-template-columns: repeat(3, 1fr); }
}

@media (max-width: 900px) {
  .sk-content-grid { grid-template-columns: 1fr; }
  .sk-actions-grid { grid-template-columns: repeat(2, 1fr); }
}

@media (max-width: 640px) {
  .sk-stats-grid { grid-template-columns: repeat(2, 1fr); }
  .sk-actions-grid { grid-template-columns: 1fr; }
}
</style>
