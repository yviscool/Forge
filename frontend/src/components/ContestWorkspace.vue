<template>
  <div class="grid gap-4 md:grid-cols-3">
    <div class="rounded border bg-white p-4 md:col-span-2">
      <div class="mb-2 flex items-center justify-between">
        <h2 class="font-bold">{{ t('problems') }}</h2>
        <div class="flex gap-1">
          <button
            v-for="p in problems"
            :key="p.id"
            class="rounded border px-2 py-1 text-xs"
            :class="{ 'bg-blue-600 text-white': active?.id === p.id }"
            @click="active = p"
          >
            {{ p.code }}. {{ p.title }}
          </button>
        </div>
      </div>
      <div v-if="active">
        <div class="mb-2 text-xs text-slate-500">
          时限 {{ active.timeLimitMs }}ms · 内存 {{ active.memoryLimitMib }}MiB ·
          <a :href="api.exportUrl(cid, active.id)" target="_blank" class="text-blue-600">CCF 排版</a>
        </div>
        <h3 class="font-bold">{{ active.code }}. {{ active.title }}</h3>
        <MarkdownView :source="active.statement" />
        <p class="mt-2 text-sm"><b>输入：</b>{{ active.input }}</p>
        <p class="text-sm"><b>输出：</b>{{ active.output }}</p>
        <pre class="mt-2 rounded bg-slate-100 p-2 text-xs">{{ active.examples }}</pre>
        <p class="mt-2 text-xs text-slate-500">{{ active.constraints }}</p>
      </div>
      <p v-else class="text-sm text-slate-400">暂无试题</p>
    </div>
    <div class="rounded border bg-white p-4">
      <h2 class="mb-2 font-bold">{{ t('submit_code') }}</h2>
      <select v-model="language" class="mb-2 w-full rounded border px-2 py-1 text-sm">
        <option value="cpp">C++ (g++ 14)</option>
        <option value="python">Python 3</option>
        <option value="go">Go</option>
      </select>
      <MonacoEditor v-model="code" :language="language" />
      <button class="mt-2 w-full rounded bg-blue-600 py-2 text-sm text-white" @click="submit">
        {{ t('submit_code') }}
      </button>
      <h3 class="mb-1 mt-4 font-bold">{{ t('ranking') }} ● 实时</h3>
      <Scoreboard :ranks="ranks" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api, type Problem, type RankEntry } from '../api/client'
import { useLocale } from '../i18n/dict'
import { useSSE } from '../realtime/useSSE'
import MonacoEditor from '../components/MonacoEditor.vue'
import MarkdownView from '../components/MarkdownView.vue'
import Scoreboard from '../components/Scoreboard.vue'

const props = defineProps<{ cid: string }>()
const { cid } = props
const { t } = useLocale()
const problems = ref<Problem[]>([])
const active = ref<Problem | null>(null)
const ranks = ref<RankEntry[]>([])
const code = ref('')
const language = ref('cpp')

async function refresh() {
  problems.value = await api.problems(cid)
  if (!active.value && problems.value.length) active.value = problems.value[0]
  ranks.value = await api.ranking(cid)
}
async function submit() {
  if (!active.value) return
  await api.submit(cid, { problemID: active.value.id, userId: 'usr-0001', language: language.value, code: code.value })
  await refresh()
}
useSSE(() => cid, () => refresh())
onMounted(refresh)
</script>
