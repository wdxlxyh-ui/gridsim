<template>
  <div class="app-logo" role="img" aria-label="GridSim" title="GridSim">
    <svg viewBox="0 0 40 40" fill="none" xmlns="http://www.w3.org/2000/svg">
      <defs>
        <linearGradient :id="`${uid}-ring`" x1="4" y1="6" x2="36" y2="34" gradientUnits="userSpaceOnUse">
          <stop offset="0" stop-color="#60a5fa" />
          <stop offset="0.55" stop-color="#8b5cf6" />
          <stop offset="1" stop-color="#3b82f6" />
        </linearGradient>
        <linearGradient :id="`${uid}-bolt`" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stop-color="#fde68a" />
          <stop offset="1" stop-color="#f59e0b" />
        </linearGradient>
        <radialGradient :id="`${uid}-core`" cx="0.5" cy="0.5" r="0.5">
          <stop offset="0" stop-color="#3b82f6" stop-opacity="0.35" />
          <stop offset="1" stop-color="#3b82f6" stop-opacity="0" />
        </radialGradient>
        <clipPath :id="`${uid}-clip`">
          <polygon points="20,3 34.5,11.5 34.5,28.5 20,37 5.5,28.5 5.5,11.5" />
        </clipPath>
      </defs>

      <polygon points="20,3 34.5,11.5 34.5,28.5 20,37 5.5,28.5 5.5,11.5" fill="#0f1623" />
      <circle cx="20" cy="20" r="14" :fill="`url(#${uid}-core)`" />

      <g :clip-path="`url(#${uid}-clip)`">
        <rect class="logo-sheen" x="-14" y="0" width="9" height="40" fill="#fff" opacity="0.14" transform="skewX(-20)" />
      </g>

      <polygon points="20,3 34.5,11.5 34.5,28.5 20,37 5.5,28.5 5.5,11.5"
        :stroke="`url(#${uid}-ring)`" stroke-width="1.6" stroke-linejoin="round" />
      <polygon points="20,9 29.5,14.5 29.5,25.5 20,31 10.5,25.5 10.5,14.5"
        stroke="#93c5fd" stroke-opacity="0.35" stroke-width="0.8" stroke-linejoin="round" />

      <g fill="#93c5fd">
        <circle cx="20" cy="3" r="1.4" />
        <circle cx="34.5" cy="11.5" r="1.1" opacity="0.7" />
        <circle cx="34.5" cy="28.5" r="1.1" opacity="0.7" />
        <circle cx="20" cy="37" r="1.4" />
        <circle cx="5.5" cy="28.5" r="1.1" opacity="0.7" />
        <circle cx="5.5" cy="11.5" r="1.1" opacity="0.7" />
      </g>

      <path class="logo-bolt" d="M22.4 11.5 L14.6 21.6 H19.6 L17.6 28.8 L25.6 18.4 H20.4 Z"
        :fill="`url(#${uid}-bolt)`" stroke="#fef3c7" stroke-opacity="0.5" stroke-width="0.5" stroke-linejoin="round" />
    </svg>
  </div>
</template>

<script setup lang="ts">
import { useId } from 'vue'
const uid = useId()
</script>

<style scoped>
.app-logo {
  width: 32px;
  height: 32px;
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: default;
  filter: drop-shadow(0 0 6px rgba(99, 130, 246, 0.35));
  transition: filter 0.3s ease, transform 0.3s ease;
}
.app-logo:hover {
  filter: drop-shadow(0 0 10px rgba(139, 92, 246, 0.6));
  transform: scale(1.06);
}
.app-logo svg { width: 100%; height: 100%; display: block; }

.logo-sheen { animation: logo-sheen 5s ease-in-out infinite; }
.logo-bolt { animation: logo-pulse 3s ease-in-out infinite; transform-origin: 20px 20px; }

@keyframes logo-sheen {
  0%, 60% { transform: translateX(0) skewX(-20deg); }
  100% { transform: translateX(70px) skewX(-20deg); }
}
@keyframes logo-pulse {
  0%, 100% { opacity: 0.88; }
  50% { opacity: 1; }
}

@media (prefers-reduced-motion: reduce) {
  .logo-sheen, .logo-bolt { animation: none; }
  .app-logo, .app-logo:hover { transition: none; transform: none; }
}
</style>
