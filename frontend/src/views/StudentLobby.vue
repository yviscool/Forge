<template>
  <div class="grid gap-2 md:grid-cols-3">
    <button
      v-for="c in contests"
      :key="c.id"
      class="rounded border bg-white p-4 text-left hover:border-blue-500"
      @click="cid = c.id"
    >
      <span class="rounded bg-slate-100 px-2 py-0.5 text-xs">{{ c.status }}</span>
      <h3 class="mt-1 font-bold">{{ c.name }}</h3>
      <p class="text-xs text-slate-500">{{ c.description || '—' }}</p>
    </button>
  </div>
  <div class="mt-4" v-if="cid">
    <ContestWorkspace :cid="cid" :key="cid" />
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api, type Contest } from '../api/client'
import ContestWorkspace from '../components/ContestWorkspace.vue'

const contests = ref<Contest[]>([])
const cid = ref<string | null>(null)
onMounted(async () => {
  contests.value = await api.contests()
})
</script>
