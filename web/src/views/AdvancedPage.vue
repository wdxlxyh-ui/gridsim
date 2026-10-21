<template>
  <div class="advanced-page">
    <section class="hero-panel">
      <div class="hero-copy">
        <div class="eyebrow"><span class="pulse-dot"></span> ADMIN CONTROL / HIGH-RISK OPERATIONS</div>
        <h1>高级操作</h1>
        <p>仅用于系统维护。所有破坏性操作都会先创建备份，并要求当前密码与精确确认词。</p>
      </div>
      <div class="hero-mark" aria-hidden="true"><el-icon><Lock /></el-icon></div>
    </section>

    <el-alert
      title="操作范围说明"
      type="warning"
      :closable="false"
      show-icon
      class="scope-alert"
    >
      <template #default>
        数据库清理只删除历史时序表；配置删除会清空实例、趋势模板、接口测试配置及业务文件，保留 users.json、数据库文件和备份目录。
      </template>
    </el-alert>

    <section class="operation-grid">
      <el-card shadow="never" class="operation-card password-card">
        <div class="card-heading">
          <div class="icon-box blue"><el-icon><Key /></el-icon></div>
          <div><h2>修改管理员密码</h2><p>修改后当前登录令牌仍可使用，后续登录需使用新密码。</p></div>
        </div>
        <el-form label-position="top" @submit.prevent>
          <el-form-item label="当前密码">
            <el-input v-model="passwordForm.currentPassword" type="password" show-password autocomplete="current-password" placeholder="输入当前密码" />
          </el-form-item>
          <el-form-item label="新密码">
            <el-input v-model="passwordForm.newPassword" type="password" show-password autocomplete="new-password" placeholder="8～128 个字符" />
          </el-form-item>
          <el-form-item label="确认新密码">
            <el-input v-model="passwordForm.confirmPassword" type="password" show-password autocomplete="new-password" placeholder="再次输入新密码" />
          </el-form-item>
          <div class="card-actions">
            <el-button type="primary" :loading="passwordLoading" @click="submitPassword">保存新密码</el-button>
          </div>
        </el-form>
      </el-card>

      <el-card shadow="never" class="operation-card danger-card">
        <div class="card-heading">
          <div class="icon-box orange"><el-icon><RefreshRight /></el-icon></div>
          <div><h2>清理历史数据库</h2><p>停止实例并清空所有实例时序历史，不删除实例配置。</p></div>
        </div>
        <div class="danger-facts">
          <span><i class="fact-dot orange-dot"></i>执行前备份 SQLite 文件</span>
          <span><i class="fact-dot"></i>清理后历史数据不可恢复</span>
        </div>
        <el-form label-position="top" @submit.prevent>
          <el-form-item label="当前密码">
            <el-input v-model="databaseForm.password" type="password" show-password autocomplete="current-password" placeholder="输入当前密码" />
          </el-form-item>
          <el-form-item :label="`输入确认词：${DATABASE_CONFIRMATION}`">
            <el-input v-model="databaseForm.confirmation" :placeholder="DATABASE_CONFIRMATION" @keyup.enter="submitDatabaseClear" />
          </el-form-item>
          <div class="card-actions">
            <el-button type="warning" :loading="databaseLoading" @click="submitDatabaseClear">清理历史数据库</el-button>
          </div>
        </el-form>
      </el-card>

      <el-card shadow="never" class="operation-card critical-card">
        <div class="card-heading">
          <div class="icon-box red"><el-icon><Delete /></el-icon></div>
          <div><h2>删除全部业务配置</h2><p>停止所有实例并清空实例、趋势、代理和业务文件配置。</p></div>
        </div>
        <div class="danger-facts">
          <span><i class="fact-dot red-dot"></i>执行前备份整个 config 目录</span>
          <span><i class="fact-dot"></i>users.json 和备份目录会保留</span>
        </div>
        <el-form label-position="top" @submit.prevent>
          <el-form-item label="当前密码">
            <el-input v-model="configForm.password" type="password" show-password autocomplete="current-password" placeholder="输入当前密码" />
          </el-form-item>
          <el-form-item :label="`输入确认词：${CONFIG_CONFIRMATION}`">
            <el-input v-model="configForm.confirmation" :placeholder="CONFIG_CONFIRMATION" @keyup.enter="submitConfigClear" />
          </el-form-item>
          <div class="card-actions">
            <el-button type="danger" :loading="configLoading" @click="submitConfigClear">删除全部业务配置</el-button>
          </div>
        </el-form>
      </el-card>
    </section>

    <transition name="result-fade">
      <div v-if="lastResult" class="result-strip" :class="lastResult.kind">
        <el-icon><CircleCheckFilled v-if="lastResult.kind === 'success'" /><WarningFilled v-else /></el-icon>
        <div><strong>{{ lastResult.title }}</strong><span>{{ lastResult.detail }}</span></div>
      </div>
    </transition>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { CircleCheckFilled, Delete, Key, Lock, RefreshRight, WarningFilled } from '@element-plus/icons-vue'
import { changeAdminPassword, clearAdminConfig, clearAdminDatabase, type AdminOperationResponse } from '../api'

const DATABASE_CONFIRMATION = 'CLEAR_DATABASE'
const CONFIG_CONFIRMATION = 'DELETE_ALL_CONFIG'

const passwordForm = reactive({ currentPassword: '', newPassword: '', confirmPassword: '' })
const databaseForm = reactive({ password: '', confirmation: '' })
const configForm = reactive({ password: '', confirmation: '' })
const passwordLoading = ref(false)
const databaseLoading = ref(false)
const configLoading = ref(false)
const lastResult = ref<{ kind: 'success' | 'error'; title: string; detail: string } | null>(null)

function errorMessage(error: any): string {
  return error?.response?.data?.error || error?.response?.data?.message || '请求未完成，请检查服务日志。'
}

function showResult(kind: 'success' | 'error', title: string, detail: string) {
  lastResult.value = { kind, title, detail }
}

async function submitPassword() {
  if (!passwordForm.currentPassword || !passwordForm.newPassword || !passwordForm.confirmPassword) {
    ElMessage.warning('请完整填写密码字段')
    return
  }
  if (passwordForm.newPassword !== passwordForm.confirmPassword) {
    ElMessage.warning('两次输入的新密码不一致')
    return
  }
  passwordLoading.value = true
  try {
    await changeAdminPassword(passwordForm.currentPassword, passwordForm.newPassword, passwordForm.confirmPassword)
    passwordForm.currentPassword = ''
    passwordForm.newPassword = ''
    passwordForm.confirmPassword = ''
    showResult('success', '管理员密码已修改', '新密码将在下一次登录时生效。')
    ElMessage.success('管理员密码已修改')
  } catch (error) {
    showResult('error', '密码修改未完成', errorMessage(error))
    ElMessage.error(errorMessage(error))
  } finally {
    passwordLoading.value = false
  }
}

async function submitDatabaseClear() {
  if (!databaseForm.password || databaseForm.confirmation !== DATABASE_CONFIRMATION) {
    ElMessage.warning(`请输入当前密码，并准确输入 ${DATABASE_CONFIRMATION}`)
    return
  }
  try {
    await ElMessageBox.confirm(
      '该操作会停止实例并清空所有历史时序数据。系统会先创建数据库备份，是否继续？',
      '确认清理历史数据库',
      { type: 'warning', confirmButtonText: '继续清理', cancelButtonText: '取消' },
    )
  } catch { return }
  databaseLoading.value = true
  try {
    const result = await clearAdminDatabase(databaseForm.password, databaseForm.confirmation)
    databaseForm.password = ''
    databaseForm.confirmation = ''
    showResult('success', '历史数据库已清理', formatOperationResult(result, '历史记录'))
    ElMessage.success('历史数据库已清理')
  } catch (error) {
    showResult('error', '数据库清理未完成', errorMessage(error))
    ElMessage.error(errorMessage(error))
  } finally {
    databaseLoading.value = false
  }
}

async function submitConfigClear() {
  if (!configForm.password || configForm.confirmation !== CONFIG_CONFIRMATION) {
    ElMessage.warning(`请输入当前密码，并准确输入 ${CONFIG_CONFIRMATION}`)
    return
  }
  try {
    await ElMessageBox.confirm(
      '该操作会删除全部业务配置，实例将被停止。系统会先备份 config 目录，是否继续？',
      '确认删除全部业务配置',
      { type: 'error', confirmButtonText: '继续删除', cancelButtonText: '取消' },
    )
  } catch { return }
  configLoading.value = true
  try {
    const result = await clearAdminConfig(configForm.password, configForm.confirmation)
    configForm.password = ''
    configForm.confirmation = ''
    showResult('success', '业务配置已清空', formatOperationResult(result, '配置文件'))
    ElMessage.success('全部业务配置已清空')
  } catch (error) {
    showResult('error', '配置删除未完成', errorMessage(error))
    ElMessage.error(errorMessage(error))
  } finally {
    configLoading.value = false
  }
}

function formatOperationResult(result: AdminOperationResponse, label: string): string {
  const count = result.deleted_rows ?? result.removed_files
  const countText = count === undefined ? '' : `已处理 ${count} 项；`
  return `${countText}备份位置：${result.backup_path || '已创建'}`
}
</script>

<style scoped>
.advanced-page { max-width: 1280px; margin: 0 auto; padding-bottom: 40px; }
.hero-panel { min-height: 156px; padding: 28px 34px; margin-bottom: 16px; position: relative; overflow: hidden; display: flex; align-items: center; justify-content: space-between; border: 1px solid rgba(245, 158, 11, .26); border-radius: 10px; background: linear-gradient(110deg, rgba(20, 29, 43, .98), rgba(35, 29, 21, .96)); box-shadow: inset 0 1px rgba(255,255,255,.04); }
.hero-panel::after { content: ''; position: absolute; width: 360px; height: 360px; right: -100px; top: -170px; border: 1px solid rgba(245,158,11,.16); border-radius: 50%; box-shadow: 0 0 0 28px rgba(245,158,11,.035), 0 0 0 58px rgba(245,158,11,.025); }
.eyebrow { color: #f4b740; font-size: 11px; letter-spacing: .16em; font-weight: 700; }
.pulse-dot { display: inline-block; width: 7px; height: 7px; margin-right: 8px; border-radius: 50%; background: #f59e0b; box-shadow: 0 0 0 4px rgba(245,158,11,.15); vertical-align: 1px; }
.hero-copy h1 { margin: 14px 0 5px; color: #f8fafc; font-size: 28px; letter-spacing: -.03em; }
.hero-copy p { margin: 0; max-width: 620px; color: #aab5c4; font-size: 13px; }
.hero-mark { position: relative; z-index: 1; margin-right: 36px; color: rgba(245,158,11,.72); font-size: 72px; transform: rotate(-10deg); }
.scope-alert { margin-bottom: 16px; }
.operation-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 16px; align-items: stretch; }
.operation-card { min-height: 470px; border-color: var(--border-color); background: var(--card-bg, var(--bg-secondary)); }
.card-heading { display: flex; align-items: flex-start; gap: 12px; min-height: 76px; margin-bottom: 19px; }
.card-heading h2 { margin: 1px 0 7px; color: var(--text-primary); font-size: 17px; }
.card-heading p { margin: 0; color: var(--text-secondary); font-size: 12px; line-height: 1.55; }
.icon-box { width: 38px; height: 38px; flex: 0 0 38px; display: grid; place-items: center; border-radius: 8px; font-size: 19px; }
.icon-box.blue { color: #60a5fa; background: rgba(59,130,246,.14); }
.icon-box.orange { color: #fbbf24; background: rgba(245,158,11,.14); }
.icon-box.red { color: #f87171; background: rgba(239,68,68,.14); }
.danger-facts { display: flex; flex-direction: column; gap: 7px; padding: 12px 13px; margin-bottom: 19px; color: var(--text-secondary); font-size: 12px; border: 1px solid rgba(148,163,184,.13); border-radius: 6px; background: rgba(15,23,42,.2); }
.fact-dot { display: inline-block; width: 6px; height: 6px; margin: 0 8px 1px 0; border-radius: 50%; background: #94a3b8; }
.orange-dot { background: #f59e0b; } .red-dot { background: #ef4444; }
.card-actions { display: flex; justify-content: flex-end; padding-top: 3px; }
.result-strip { display: flex; align-items: center; gap: 11px; margin-top: 16px; padding: 13px 16px; border: 1px solid; border-radius: 7px; font-size: 13px; }
.result-strip.success { color: #86efac; border-color: rgba(34,197,94,.28); background: rgba(22,101,52,.17); }
.result-strip.error { color: #fca5a5; border-color: rgba(239,68,68,.3); background: rgba(127,29,29,.17); }
.result-strip strong { display: block; margin-bottom: 3px; } .result-strip span { color: var(--text-secondary); font-size: 12px; }
.result-fade-enter-active, .result-fade-leave-active { transition: opacity .2s, transform .2s; } .result-fade-enter-from, .result-fade-leave-to { opacity: 0; transform: translateY(-5px); }
@media (max-width: 1050px) { .operation-grid { grid-template-columns: 1fr; } .operation-card { min-height: auto; } }
@media (max-width: 600px) { .hero-panel { padding: 22px; } .hero-mark { display: none; } .hero-copy h1 { font-size: 24px; } }
</style>
