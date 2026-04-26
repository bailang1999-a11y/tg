import { botApi } from './bot'
import { coreApi } from './core'
import { listenerApi } from './listener'
import { scrmApi } from './scrm'
import { taskApi } from './tasks'

export const api = {
  ...coreApi,
  ...listenerApi,
  ...taskApi,
  ...scrmApi,
  ...botApi
}

export { botApi, coreApi, listenerApi, scrmApi, taskApi }
