import React, { lazy, Suspense } from 'react'
import { Navigate, Route, Routes, useLocation } from 'react-router-dom'
import { AppShell } from './app/AppShell'
import { ErrorBoundary } from './app/ErrorBoundary'
import { SessionGate } from './app/SessionGate'
import { RequireEducator, RequireSession } from './app/ProtectedRoute'
import { ToastProvider } from './components/ui/toast'
import { LoadingBlock } from './components/ui/empty-state'

const LoginPage = lazy(() => import('./pages/LoginPage').then((module) => ({ default: module.LoginPage })))
const TrayPanel = lazy(() =>
  import('./pages/TrayPanel').then((module) => ({ default: module.TrayPanel }))
)
const StudentLobby = lazy(() =>
  import('./views/StudentLobby').then((module) => ({ default: module.StudentLobby }))
)
const StudentWorkspace = lazy(() =>
  import('./views/StudentWorkspace').then((module) => ({ default: module.StudentWorkspace }))
)
const TeacherConsole = lazy(() =>
  import('./views/TeacherConsole').then((module) => ({ default: module.TeacherConsole }))
)

export function App() {
  const location = useLocation()
  return (
    <ToastProvider>
      {/* 托盘速览面板：自带外框与登录，不套站点外壳 */}
      {location.pathname === '/panel' ? (
        <Suspense fallback={<LoadingBlock label="正在载入监考面板..." />}>
          <TrayPanel />
        </Suspense>
      ) : (
      <SessionGate>
        <AppShell>
          <ErrorBoundary key={location.pathname}>
            <Suspense fallback={<LoadingBlock label="正在载入工作区..." />}>
              <Routes>
                <Route path="/login" element={<LoginPage />} />
                <Route
                  path="/"
                  element={
                    <RequireSession>
                      <StudentLobby />
                    </RequireSession>
                  }
                />
                <Route
                  path="/contest/:cid"
                  element={
                    <RequireSession>
                      <StudentWorkspace />
                    </RequireSession>
                  }
                />
                <Route
                  path="/teacher/*"
                  element={
                    <RequireEducator>
                      <TeacherConsole />
                    </RequireEducator>
                  }
                />
                <Route path="*" element={<Navigate to="/" replace />} />
              </Routes>
            </Suspense>
          </ErrorBoundary>
        </AppShell>
      </SessionGate>
      )}
    </ToastProvider>
  )
}
