// Lightweight editor stand-in for jsdom tests. Production always resolves the
// real monaco-editor package during the Vite build.
export const editor = {
  create: () => ({
    dispose() {},
    getModel: () => null,
    getValue: () => '',
    onDidChangeModelContent: () => ({ dispose() {} }),
    setValue() {},
    // 挂载与 props 同步（语言/字号）时会被调用，缺失会让组件树卸载。
    updateOptions() {},
    layout() {},
  }),
  createDiffEditor: () => ({
    dispose() {},
    getModel: () => null,
    setModel() {},
  }),
  createModel: () => ({
    dispose() {},
    getValue: () => '',
    setValue() {},
  }),
  setModelLanguage() {},
}
