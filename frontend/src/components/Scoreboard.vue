<template>
  <table class="w-full text-sm">
    <thead>
      <tr class="border-b text-left text-slate-500">
        <th class="py-1">#</th>
        <th>选手</th>
        <th>总分</th>
        <th>AC</th>
      </tr>
    </thead>
    <tbody>
      <tr v-for="(r, i) in ranks" :key="r.userName" class="border-b">
        <td class="py-1 font-bold">#{{ i + 1 }}</td>
        <td>{{ r.userName }}</td>
        <td class="font-bold text-blue-600">{{ r.score }}</td>
        <td>{{ r.accepted }}</td>
      </tr>
    </tbody>
  </table>
</template>

<script setup lang="ts">
import type { RankEntry } from '../api/client'
defineProps<{ ranks: RankEntry[] }>()
</script>
