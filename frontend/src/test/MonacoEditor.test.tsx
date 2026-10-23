import { act, render } from '@testing-library/react'
import * as monaco from 'monaco-editor/esm/vs/editor/editor.api'
import { describe, expect, it, vi } from 'vitest'
import { MonacoEditor } from '../components/MonacoEditor'

describe('MonacoEditor', () => {
  it('creates one editor, syncs props, and disposes its listener', async () => {
    const onChange = vi.fn()
    const disposeListener = vi.fn()
    const disposeEditor = vi.fn()
    const updateOptions = vi.fn()
    const setValue = vi.fn()
    let currentValue = 'initial'
    let changeHandler: (() => void) | undefined
    const model = {} as monaco.editor.ITextModel
    const created = {
      dispose: disposeEditor,
      getModel: vi.fn(() => model),
      getValue: vi.fn(() => currentValue),
      onDidChangeModelContent: vi.fn((handler: () => void) => {
        changeHandler = handler
        return { dispose: disposeListener }
      }),
      setValue,
      updateOptions,
    } as unknown as monaco.editor.IStandaloneCodeEditor
    const create = vi.spyOn(monaco.editor, 'create').mockReturnValue(created)
    const setModelLanguage = vi.spyOn(monaco.editor, 'setModelLanguage')

    const view = render(
      <MonacoEditor
        value="initial"
        onChange={onChange}
        language="python"
        readOnly={false}
        height="300px"
        ariaLabel="Python source"
      />
    )

    expect(create).toHaveBeenCalledOnce()
    expect(create).toHaveBeenCalledWith(
      expect.any(HTMLDivElement),
      expect.objectContaining({
        value: 'initial',
        language: 'python',
        readOnly: false,
        ariaLabel: 'Python source',
      })
    )
    expect(create.mock.calls[0][1]?.fontFamily).toContain('monospace')

    currentValue = 'typed'
    act(() => changeHandler?.())
    expect(onChange).toHaveBeenCalledWith('typed')

    view.rerender(
      <MonacoEditor
        value="external"
        onChange={onChange}
        language="go"
        readOnly
        height="300px"
        ariaLabel="Python source"
      />
    )
    expect(setValue).toHaveBeenCalledWith('external')
    expect(updateOptions).toHaveBeenCalledWith({ readOnly: true })
    await act(async () => {
      await Promise.resolve()
    })
    expect(setModelLanguage).toHaveBeenCalledWith(model, 'go')

    view.unmount()
    expect(disposeListener).toHaveBeenCalledOnce()
    expect(disposeEditor).toHaveBeenCalledOnce()
  })
})
