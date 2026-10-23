// Lightweight editor stand-in for jsdom tests. Production always resolves the
// real monaco-editor package during the Vite build.
export const editor = {
  create: () => ({
    dispose() {},
    getModel: () => null,
    getValue: () => '',
    onDidChangeModelContent: () => ({ dispose() {} }),
    setValue() {},
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
