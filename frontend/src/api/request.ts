const API_BASE = import.meta.env.VITE_API_BASE_URL ?? ''

type ApiEnvelope<T> = {
  data?: T
  error?: string
}

export async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const token = localStorage.getItem('codex3_token')
  const headers = new Headers(options.headers)
  if (!headers.has('Content-Type') && options.body && !(options.body instanceof FormData)) {
    headers.set('Content-Type', 'application/json')
  }
  if (token) {
    headers.set('Authorization', `Bearer ${token}`)
  }

  const response = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers
  })
  const payload = (await response.json().catch(() => ({}))) as ApiEnvelope<T>
  if (!response.ok) {
    throw new Error(payload.error || '请求失败')
  }
  return payload.data as T
}
