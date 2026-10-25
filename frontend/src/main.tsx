import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { App } from './App'
import '@fontsource-variable/manrope'
import '@fontsource-variable/jetbrains-mono'
import '@fontsource-variable/fraunces'
import './style.css'
import 'katex/dist/katex.min.css'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: 1,
    },
  },
})

// 开发预览：?desktop-preview 直接渲染考场桌面（不进生产构建）
if (import.meta.env.DEV && new URLSearchParams(window.location.search).has('desktop-preview')) {
  void import('./dev/DesktopPreview').then(({ DesktopPreview }) => {
    ReactDOM.createRoot(document.getElementById('root')!).render(<DesktopPreview />)
  })
} else {
ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter basename="/app">
        <App />
      </BrowserRouter>
    </QueryClientProvider>
  </React.StrictMode>
)
}
