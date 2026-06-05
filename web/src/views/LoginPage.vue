<template>
  <div class="login-wrapper">
    <!-- Aurora background -->
    <div class="aurora-bg">
      <div class="aurora aurora-1"></div>
      <div class="aurora aurora-2"></div>
      <div class="aurora aurora-3"></div>
    </div>
    <!-- Noise overlay -->
    <div class="noise-overlay"></div>

    <div class="login-card">
      <div class="login-header">
        <div class="login-logo">
          <svg viewBox="0 0 56 56" fill="none" xmlns="http://www.w3.org/2000/svg">
            <!-- Outer hexagon (grid topology) -->
            <polygon points="28,4 50,16 50,40 28,52 6,40 6,16"
              stroke="url(#logo-grad)" stroke-width="1.5" fill="none" opacity="0.4"/>
            <!-- Inner hexagon -->
            <polygon points="28,14 40,21 40,35 28,42 16,35 16,21"
              stroke="#60a5fa" stroke-width="1" fill="rgba(59,130,246,0.08)"/>
            <!-- Hex vertex dots (substations) -->
            <circle cx="28" cy="4" r="2" fill="#60a5fa" opacity="0.7"/>
            <circle cx="50" cy="16" r="2" fill="#60a5fa" opacity="0.7"/>
            <circle cx="50" cy="40" r="2" fill="#60a5fa" opacity="0.7"/>
            <circle cx="28" cy="52" r="2" fill="#60a5fa" opacity="0.7"/>
            <circle cx="6" cy="40" r="2" fill="#60a5fa" opacity="0.7"/>
            <circle cx="6" cy="16" r="2" fill="#60a5fa" opacity="0.7"/>
            <!-- Cross-links (transmission lines) -->
            <line x1="28" y1="14" x2="28" y2="42" stroke="#3b82f6" stroke-width="0.8" opacity="0.3"/>
            <line x1="16" y1="21" x2="40" y2="35" stroke="#3b82f6" stroke-width="0.8" opacity="0.3"/>
            <line x1="16" y1="35" x2="40" y2="21" stroke="#3b82f6" stroke-width="0.8" opacity="0.3"/>
            <!-- Lightning bolt (power/energy) -->
            <path d="M26,20 L22,28 L26,28 L23,36" stroke="#f59e0b" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" fill="none">
              <animate attributeName="opacity" values="0.6;1;0.6" dur="2s" repeatCount="indefinite"/>
            </path>
            <!-- Signal waves (IEC104 communication) -->
            <path d="M32,26 Q35,23 38,26" stroke="#93c5fd" stroke-width="1.2" fill="none" stroke-linecap="round" opacity="0.6"/>
            <path d="M33,30 Q37,26 41,30" stroke="#93c5fd" stroke-width="1" fill="none" stroke-linecap="round" opacity="0.4"/>
            <defs>
              <linearGradient id="logo-grad" x1="6" y1="16" x2="50" y2="40">
                <stop offset="0%" stop-color="#3b82f6"/>
                <stop offset="50%" stop-color="#8b5cf6"/>
                <stop offset="100%" stop-color="#3b82f6"/>
              </linearGradient>
            </defs>
          </svg>
        </div>
        <h2 class="split-title">
          <span v-for="(char, i) in titleChars" :key="i" class="split-char" :style="{ animationDelay: `${i * 60}ms` }">{{ char === ' ' ? '\u00A0' : char }}</span>
        </h2>
        <p class="login-subtitle slide-up" style="animation-delay: 0.6s">请登录以继续</p>
      </div>
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        class="login-form slide-up"
        style="animation-delay: 0.75s"
        @keyup.enter="handleLogin"
      >
        <el-form-item prop="username">
          <el-input
            v-model="form.username"
            placeholder="用户名"
            :prefix-icon="User"
            size="large"
            autocomplete="username"
          />
        </el-form-item>
        <el-form-item prop="password">
          <el-input
            v-model="form.password"
            type="password"
            placeholder="密码"
            :prefix-icon="Lock"
            size="large"
            show-password
            autocomplete="current-password"
          />
        </el-form-item>
        <el-form-item>
          <el-button
            type="primary"
            size="large"
            :loading="loading"
            style="width: 100%"
            @click="handleLogin"
          >
            {{ loading ? '登录中...' : '登 录' }}
          </el-button>
        </el-form-item>
        <div v-if="errorMsg" class="login-error">{{ errorMsg }}</div>
      </el-form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { User, Lock } from '@element-plus/icons-vue'
import { login, setToken } from '../api'
import type { FormInstance, FormRules } from 'element-plus'

const router = useRouter()
const formRef = ref<FormInstance>()
const loading = ref(false)
const errorMsg = ref('')

const titleChars = 'IEC104 模拟器管理'.split('')

const form = reactive({
  username: '',
  password: '',
})

const rules: FormRules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }],
}

async function handleLogin() {
  if (!formRef.value) return
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  loading.value = true
  errorMsg.value = ''

  try {
    const res = await login(form.username, form.password)
    setToken(res.token)
    router.push('/config')
  } catch (err: any) {
    errorMsg.value = err?.response?.data?.error || '登录失败，请检查用户名和密码'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
/* ─── Aurora background ─── */
.login-wrapper {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100vh;
  overflow: hidden;
  background: #0a0e17;
}

.aurora-bg {
  position: absolute;
  inset: 0;
  overflow: hidden;
  z-index: 0;
}

.aurora {
  position: absolute;
  border-radius: 50%;
  filter: blur(80px);
  opacity: 0.5;
  animation: aurora-drift 8s ease-in-out infinite alternate;
}

.aurora-1 {
  width: 500px;
  height: 500px;
  background: radial-gradient(circle, rgba(59,130,246,0.4), transparent 70%);
  top: -10%;
  left: -10%;
  animation-duration: 10s;
}

.aurora-2 {
  width: 400px;
  height: 400px;
  background: radial-gradient(circle, rgba(139,92,246,0.3), transparent 70%);
  bottom: -15%;
  right: -5%;
  animation-duration: 12s;
  animation-delay: -3s;
}

.aurora-3 {
  width: 350px;
  height: 350px;
  background: radial-gradient(circle, rgba(6,182,212,0.25), transparent 70%);
  top: 40%;
  left: 50%;
  animation-duration: 9s;
  animation-delay: -5s;
}

@keyframes aurora-drift {
  0%   { transform: translate(0, 0) scale(1); }
  33%  { transform: translate(60px, -40px) scale(1.1); }
  66%  { transform: translate(-30px, 50px) scale(0.95); }
  100% { transform: translate(40px, 20px) scale(1.05); }
}

/* ─── Noise overlay ─── */
.noise-overlay {
  position: absolute;
  inset: 0;
  z-index: 1;
  opacity: 0.03;
  pointer-events: none;
  background-image: url("data:image/svg+xml,%3Csvg viewBox='0 0 256 256' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='noise'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.9' numOctaves='4' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23noise)'/%3E%3C/svg%3E");
  background-repeat: repeat;
  background-size: 256px 256px;
}

/* ─── Login card ─── */
.login-card {
  position: relative;
  z-index: 2;
  width: 400px;
  padding: 48px 36px 36px;
  background: rgba(17, 24, 39, 0.85);
  border: 1px solid rgba(30, 41, 59, 0.8);
  border-radius: 16px;
  box-shadow: 0 8px 40px rgba(0, 0, 0, 0.5), 0 0 80px rgba(59,130,246,0.06);
  backdrop-filter: blur(20px);
}

.login-header {
  text-align: center;
  margin-bottom: 32px;
}

.login-logo {
  width: 56px;
  height: 56px;
  margin: 0 auto 16px;
  animation: logo-glow 3s ease-in-out infinite alternate;
}

.login-logo svg {
  width: 56px;
  height: 56px;
}

@keyframes logo-glow {
  0%   { filter: drop-shadow(0 0 8px rgba(59,130,246,0.3)); }
  100% { filter: drop-shadow(0 0 16px rgba(59,130,246,0.5)); }
}

/* ─── SplitText title ─── */
.split-title {
  margin: 0;
  font-size: 22px;
  font-weight: 700;
  color: #e2e8f0;
  display: flex;
  justify-content: center;
  flex-wrap: wrap;
  gap: 1px;
}

.split-char {
  display: inline-block;
  opacity: 0;
  transform: translateY(12px);
  animation: char-in 0.5s cubic-bezier(0.22, 1, 0.36, 1) forwards;
}

@keyframes char-in {
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

/* ─── Slide-up animation ─── */
.slide-up {
  opacity: 0;
  transform: translateY(16px);
  animation: slide-up-in 0.6s cubic-bezier(0.22, 1, 0.36, 1) forwards;
}

@keyframes slide-up-in {
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

/* ─── Form styles ─── */
.login-subtitle {
  margin: 8px 0 0;
  font-size: 14px;
  color: #64748b;
}

.login-form {
  margin-top: 8px;
}

.login-form :deep(.el-input__wrapper) {
  background: rgba(10, 14, 23, 0.8);
  box-shadow: 0 0 0 1px #1e293b inset;
  border-radius: 8px;
  transition: box-shadow 0.25s, background 0.25s;
}

.login-form :deep(.el-input__wrapper.is-focus) {
  box-shadow: 0 0 0 1px #3b82f6 inset, 0 0 12px rgba(59,130,246,0.15);
  background: rgba(10, 14, 23, 1);
}

.login-form :deep(.el-input__inner) {
  color: #e2e8f0;
  height: 42px;
}

.login-form :deep(.el-input__inner::placeholder) {
  color: #64748b;
}

.login-form :deep(.el-input__prefix-inner) {
  color: #64748b;
}

.login-form :deep(.el-button--primary) {
  height: 44px;
  font-size: 15px;
  letter-spacing: 2px;
  background: linear-gradient(135deg, #3b82f6, #2563eb);
  border: none;
  border-radius: 8px;
  transition: transform 0.15s, box-shadow 0.25s, opacity 0.2s;
}

.login-form :deep(.el-button--primary:hover) {
  background: linear-gradient(135deg, #60a5fa, #3b82f6);
  box-shadow: 0 4px 20px rgba(59, 130, 246, 0.35);
  transform: translateY(-1px);
}

.login-form :deep(.el-button--primary:active) {
  transform: translateY(0);
  box-shadow: 0 2px 8px rgba(59, 130, 246, 0.25);
}

.login-error {
  text-align: center;
  color: #ef4444;
  font-size: 13px;
  padding: 8px;
  background: rgba(239, 68, 68, 0.1);
  border-radius: 6px;
  border: 1px solid rgba(239, 68, 68, 0.2);
  animation: shake 0.4s ease;
}

@keyframes shake {
  0%, 100% { transform: translateX(0); }
  20%      { transform: translateX(-6px); }
  40%      { transform: translateX(6px); }
  60%      { transform: translateX(-4px); }
  80%      { transform: translateX(4px); }
}
</style>
