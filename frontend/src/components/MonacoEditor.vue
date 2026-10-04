<template>
  <div ref="el" class="h-64 w-full rounded border"></div>
</template>

<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import * as monaco from 'monaco-editor'

const props = defineProps<{ modelValue: string; language?: string }>()
const emit = defineEmits(['update:modelValue'])
const el = ref<HTMLDivElement | null>(null)
let editor: monaco.editor.IStandaloneCodeEditor | null = null

onMounted(() => {
  editor = monaco.editor.create(el.value!, {
    value: props.modelValue || '// paste your code here',
    language: props.language || 'cpp',
    minimap: { enabled: false },
    fontSize: 13,
  })
  editor.onDidChangeModelContent(() => emit('update:modelValue', editor!.getValue()))
})

watch(
  () => props.language,
  (lang) => {
    if (editor) monaco.editor.setModelLanguage(editor.getModel()!, lang || 'cpp')
  },
)
</script>
