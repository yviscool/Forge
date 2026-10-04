<template>
  <div class="min-h-screen bg-slate-50 text-slate-900">
    <header class="border-b bg-white">
      <div class="mx-auto flex max-w-6xl items-center justify-between px-4 py-3">
        <div class="flex items-center gap-2">
          <span>⚡</span>
          <div>
            <h1 class="text-lg font-bold">{{ t('brand') }}</h1>
            <p class="text-xs text-slate-500">{{ t('subtitle') }}</p>
          </div>
        </div>
        <div class="flex items-center gap-2">
          <button class="rounded border px-3 py-1 text-sm" @click="toggle">
            {{ locale === 'zh-CN' ? 'English' : '中文' }}
          </button>
          <router-link v-if="$route.path !== '/teacher'" to="/teacher" class="rounded bg-blue-600 px-3 py-1 text-sm text-white">
            {{ t('teacher') }} →
          </router-link>
          <router-link v-else to="/" class="rounded bg-blue-600 px-3 py-1 text-sm text-white">← {{ t('title') }}</router-link>
        </div>
      </div>
    </header>
    <main class="mx-auto max-w-6xl px-4 py-4">
      <router-view />
    </main>
  </div>
</template>

<script setup lang="ts">
import { useLocale } from './i18n/dict'
const { locale, t, toggle } = useLocale()
</script>
