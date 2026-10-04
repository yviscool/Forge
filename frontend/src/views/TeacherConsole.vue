<template>
  <div class="mb-3 flex items-center gap-2 rounded border bg-white p-3">
    <select v-model="cid" class="rounded border px-2 py-1 text-sm">
      <option v-for="c in contests" :key="c.id" :value="c.id">{{ c.name }} [{{ c.status }}]</option>
    </select>
    <button class="rounded bg-green-600 px-3 py-1 text-sm text-white" @click="start">{{ t('start_contest') }}</button>
    <button class="rounded bg-red-600 px-3 py-1 text-sm text-white" @click="finish">{{ t('finish_contest') }}</button>
    <button class="rounded border px-3 py-1 text-sm" @click="showCreate = !showCreate">+ 比赛</button>
  </div>

  <div v-if="showCreate" class="mb-3 flex gap-2 rounded border bg-white p-3">
    <input v-model="form.name" placeholder="比赛名称" class="flex-1 rounded border px-2 py-1 text-sm" />
    <input v-model="form.description" placeholder="说明" class="flex-1 rounded border px-2 py-1 text-sm" />
    <button class="rounded bg-blue-600 px-3 py-1 text-sm text-white" @click="create">创建</button>
  </div>

  <div class="grid gap-4 md:grid-cols-2">
    <div class="rounded border bg-white p-4">
      <h3 class="mb-2 font-bold">试题 · 校验导出</h3>
      <div class="mb-2 flex gap-2">
        <input v-model="pform.code" placeholder="代号 A" class="w-20 rounded border px-2 py-1 text-sm" />
        <input v-model="pform.title" placeholder="题目名称" class="flex-1 rounded border px-2 py-1 text-sm" />
        <button class="rounded bg-blue-600 px-3 py-1 text-sm text-white" @click="createProblem">保存</button>
      </div>
      <textarea v-model="pform.statement" placeholder="题目描述 (Markdown + $公式$)" rows="3" class="mb-1 w-full rounded border px-2 py-1 text-sm"></textarea>
      <div class="grid grid-cols-2 gap-1">
        <input v-model="pform.input" placeholder="输入格式" class="rounded border px-2 py-1 text-sm" />
        <input v-model="pform.output" placeholder="输出格式" class="rounded border px-2 py-1 text-sm" />
        <input v-model="pform.examples" placeholder="样例" class="rounded border px-2 py-1 text-sm" />
        <input v-model="pform.constraints" placeholder="数据范围" class="rounded border px-2 py-1 text-sm" />
      </div>
      <ul class="mt-2 space-y-1">
        <li v-for="p in problems" :key="p.id" class="flex items-center justify-between rounded bg-slate-50 px-2 py-1 text-sm">
          <span>{{ p.code }}. {{ p.title }}</span>
          <span class="flex gap-1">
            <button class="rounded border px-2 text-xs" @click="validate(p.id)">校验</button>
            <a class="rounded border px-2 text-xs" :href="api.exportUrl(cid, p.id)" target="_blank">CCF 预览</a>
          </span>
        </li>
      </ul>
      <p v-if="validation" class="mt-1 text-xs" :class="validation.ok ? 'text-green-600' : 'text-red-600'">{{ validation.msg }}</p>
    </div>
    <div class="rounded border bg-white p-4">
      <h3 class="mb-2 font-bold">{{ t('ranking') }} ● 实时</h3>
      <Scoreboard :ranks="ranks" />
      <h3 class="mb-1 mt-3 font-bold">提交流（教师可判）</h3>
      <ul class="space-y-1 text-sm">
        <li v-for="s in subs" :key="s.id" class="flex items-center justify-between rounded bg-slate-50 px-2 py-1">
          <span>{{ s.userName }} · {{ s.language }} · {{ s.verdict }} · {{ s.score }}</span>
          <span class="flex gap-1">
            <button class="rounded border px-2 text-xs" @click="judge(s.id, 'accepted', 100)">通过</button>
            <button class="rounded border px-2 text-xs" @click="judge(s.id, 'wrong_answer', 0)">错误</button>
          </span>
        </li>
      </ul>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { api, type Contest, type Problem, type RankEntry, type Submission } from '../api/client'
import { useLocale } from '../i18n/dict'
import { useSSE } from '../realtime/useSSE'
import Scoreboard from '../components/Scoreboard.vue'

const { t } = useLocale()
const contests = ref<Contest[]>([])
const problems = ref<Problem[]>([])
const ranks = ref<RankEntry[]>([])
const subs = ref<Submission[]>([])
const cid = ref('')
const showCreate = ref(false)
const form = ref({ name: '', description: '' })
const pform = ref({ code: '', title: '', statement: '', input: '', output: '', examples: '', constraints: '' })
const validation = ref<{ ok: boolean; msg: string } | null>(null)

async function refreshContests() {
  contests.value = await api.contests()
  if (!cid.value && contests.value.length) cid.value = contests.value[0].id
}
async function refresh() {
  if (!cid.value) return
  problems.value = await api.problems(cid.value)
  ranks.value = await api.ranking(cid.value)
  subs.value = await api.submissions(cid.value)
}
async function create() {
  await api.createContest(form.value.name, form.value.description)
  form.value = { name: '', description: '' }
  showCreate.value = false
  await refreshContests()
}
async function start() {
  await api.startContest(cid.value)
  await refreshContests()
}
async function finish() {
  await api.finishContest(cid.value)
  await refreshContests()
}
async function createProblem() {
  await api.createProblem(cid.value, { ...pform.value, timeLimitMs: 1000, memoryLimitMib: 512 })
  pform.value = { code: '', title: '', statement: '', input: '', output: '', examples: '', constraints: '' }
  await refresh()
}
async function validate(pid: string) {
  const r = await api.validateProblem(cid.value, pid)
  validation.value = r.valid ? { ok: true, msg: '✅ 题面校验通过' } : { ok: false, msg: `❌ ${r.error}` }
}
async function judge(id: string, verdict: string, score: number) {
  await api.judge(id, verdict, score)
  await refresh()
}
watch(cid, refresh)
useSSE(() => cid.value, () => refresh())
onMounted(async () => {
  await refreshContests()
  await refresh()
})
</script>
