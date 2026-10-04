<template>
  <div class="prose max-w-none" v-html="html"></div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import MarkdownIt from 'markdown-it'
import katex from 'katex'

const props = defineProps<{ source: string }>()
const md = new MarkdownIt({
  html: false,
  highlight: (s) => `<pre>${s.replace(/</g, '&lt;')}</pre>`,
})

// $...$ 行内公式渲染（轻量实现，复杂公式走 KaTeX 全量由构建时处理）。
const html = computed(() => {
  const rendered = md.render(props.source || '')
  return rendered.replace(/\$(.+?)\$/g, (_, tex) => {
    try {
      return katex.renderToString(tex)
    } catch {
      return tex
    }
  })
})
</script>
