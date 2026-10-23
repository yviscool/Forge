import React from 'react'
import { Navigate, useLocation } from 'react-router-dom'
import { useStore } from '../store/useStore'
import { LoadingBlock } from '../components/ui/empty-state'

const educatorRoles = new Set(['admin', 'teacher'])

export function RequireSession({ children }: { children: React.ReactNode }) {
  const { session, authStatus } = useStore()
  const location = useLocation()

  if (authStatus === 'idle' || authStatus === 'checking') {
    return <LoadingBlock label="正在验证登录状态..." />
  }
  if (!session) {
    return <Navigate to="/login" replace state={{ from: location.pathname }} />
  }
  return <>{children}</>
}

export function RequireEducator({ children }: { children: React.ReactNode }) {
  const { session, authStatus } = useStore()
  const location = useLocation()

  if (authStatus === 'idle' || authStatus === 'checking') {
    return <LoadingBlock label="正在验证教师权限..." />
  }
  if (!session) {
    return <Navigate to="/login" replace state={{ from: location.pathname }} />
  }
  if (!educatorRoles.has(session.role)) {
    return <Navigate to="/" replace />
  }
  return <>{children}</>
}
