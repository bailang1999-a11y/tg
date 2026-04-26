import { defineStore } from 'pinia'
import { api, type User } from '../api/client'

type State = {
  token: string
  user: User | null
  bootstrapped: boolean
}

export const useUserStore = defineStore('user', {
  state: (): State => ({
    token: localStorage.getItem('codex3_token') || '',
    user: JSON.parse(localStorage.getItem('codex3_user') || 'null') as User | null,
    bootstrapped: false
  }),
  actions: {
    async login(username: string, password: string) {
      const data = await api.login(username, password)
      this.token = data.token
      this.user = data.user
      localStorage.setItem('codex3_token', data.token)
      localStorage.setItem('codex3_user', JSON.stringify(data.user))
    },
    async bootstrap() {
      if (!this.token || this.bootstrapped) {
        this.bootstrapped = true
        return
      }
      try {
        this.user = await api.me()
        localStorage.setItem('codex3_user', JSON.stringify(this.user))
      } catch {
        this.logout()
      } finally {
        this.bootstrapped = true
      }
    },
    logout() {
      this.token = ''
      this.user = null
      localStorage.removeItem('codex3_token')
      localStorage.removeItem('codex3_user')
    }
  }
})

