import { render, screen } from '@testing-library/react'
import * as monaco from 'monaco-editor/esm/vs/editor/editor.api'
import { describe, expect, it, vi } from 'vitest'
import { CodeDiffViewer } from '../components/CodeDiffViewer'

describe('CodeDiffViewer', () => {
  it('creates a read-only Monaco diff and renders labelled revisions', () => {
    const createDiffEditor = vi.spyOn(monaco.editor, 'createDiffEditor')
    const createModel = vi.spyOn(monaco.editor, 'createModel')
    const { rerender } = render(
      <CodeDiffViewer
        original={'int main() { return 1; }'}
        modified={'int main() { return 0; }'}
        originalTitle="提交 #12"
        modifiedTitle="提交 #13"
        height="320px"
      />
    )

    expect(createDiffEditor).toHaveBeenCalledOnce()
    expect(createDiffEditor.mock.calls[0][1]).toMatchObject({
      readOnly: true,
      renderSideBySide: true,
      minimap: { enabled: false },
    })
    expect(createModel).toHaveBeenCalledWith('int main() { return 1; }', 'cpp')
    expect(createModel).toHaveBeenCalledWith('int main() { return 0; }', 'cpp')
    expect(screen.getByText(/提交 #12/)).toBeInTheDocument()
    expect(screen.getByText(/提交 #13/)).toBeInTheDocument()

    rerender(
      <CodeDiffViewer
        original={'print(1)'}
        modified={'print(2)'}
        language="python"
        height="240px"
      />
    )
    expect(screen.getByText(/上一版本/)).toBeInTheDocument()
    expect(screen.getByText(/当前版本/)).toBeInTheDocument()
  })
})
