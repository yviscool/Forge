import { http, HttpResponse, type HttpResponseResolver, type JsonBodyType } from 'msw'
import { setupServer } from 'msw/node'

export const TEST_ORIGIN = 'http://forge.test'
export const API_PREFIX = `${TEST_ORIGIN}/api/v1`

export const server = setupServer()

export function apiUrl(path: string): string {
  return `${API_PREFIX}${path.startsWith('/') ? path : `/${path}`}`
}

/**
 * The application uses root-relative URLs. Node's fetch requires absolute
 * URLs, so resolve them against a stable test origin before MSW intercepts.
 */
export function installRelativeFetch(): void {
  const nativeFetch = globalThis.fetch

  globalThis.fetch = ((input: RequestInfo | URL, init?: RequestInit) => {
    const resolved =
      typeof input === 'string' && input.startsWith('/')
        ? new URL(input, TEST_ORIGIN).toString()
        : input
    return nativeFetch(resolved, init)
  }) as typeof fetch
}

export const apiRoutes = {
  get: (path: string, resolver: HttpResponseResolver) => http.get(apiUrl(path), resolver),
  post: (path: string, resolver: HttpResponseResolver) => http.post(apiUrl(path), resolver),
  put: (path: string, resolver: HttpResponseResolver) => http.put(apiUrl(path), resolver),
  patch: (path: string, resolver: HttpResponseResolver) => http.patch(apiUrl(path), resolver),
  delete: (path: string, resolver: HttpResponseResolver) =>
    http.delete(apiUrl(path), resolver),
}

export function jsonOk<T extends JsonBodyType>(body: T, init: ResponseInit = {}): Response {
  return HttpResponse.json(body, init) as Response
}

export function jsonError(message: string, status = 400, code?: string): Response {
  return HttpResponse.json(code ? { message, code } : { message }, {
    status,
  }) as Response
}

export { HttpResponse }
