import http from 'k6/http'
import { check, sleep } from 'k6'
import ws from 'k6/ws'

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080'
const WS_URL = BASE_URL.replace(/^http/, 'ws')
const TOKEN = __ENV.TOKEN || ''

export const options = {
  scenarios: {
    api_read_mix: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '2m', target: Number(__ENV.API_VUS || 1000) },
        { duration: '5m', target: Number(__ENV.API_VUS || 1000) },
        { duration: '2m', target: 0 }
      ],
      exec: 'apiReadMix'
    },
    websocket_logs: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '2m', target: Number(__ENV.WS_VUS || 500) },
        { duration: '5m', target: Number(__ENV.WS_VUS || 500) },
        { duration: '2m', target: 0 }
      ],
      exec: 'websocketLogs'
    }
  },
  thresholds: {
    http_req_failed: ['rate<0.02'],
    http_req_duration: ['p(95)<800', 'p(99)<2000'],
    ws_session_duration: ['p(95)>2500']
  }
}

function headers() {
  return TOKEN ? { Authorization: `Bearer ${TOKEN}` } : {}
}

export function apiReadMix() {
  const params = { headers: headers() }
  const endpoints = [
    '/health',
    '/api/v1/dashboard',
    '/api/v1/tasks?limit=50',
    '/api/v1/logs?limit=50',
    '/api/v1/terminals',
    '/api/v1/targets'
  ]
  const path = endpoints[Math.floor(Math.random() * endpoints.length)]
  const res = http.get(`${BASE_URL}${path}`, params)
  check(res, {
    'status is acceptable': (r) => [200, 204, 401, 503].includes(r.status)
  })
  sleep(Math.random() * 2)
}

export function websocketLogs() {
  if (!TOKEN) {
    sleep(1)
    return
  }
  const url = `${WS_URL}/api/v1/ws/logs?access_token=${encodeURIComponent(TOKEN)}`
  ws.connect(url, {}, (socket) => {
    socket.setTimeout(() => socket.close(), 5000)
  })
}
