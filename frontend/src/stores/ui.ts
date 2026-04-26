import { defineStore } from 'pinia'

export type ToastTone = 'success' | 'error' | 'warning' | 'info'

export type ToastItem = {
  id: number
  title?: string
  message: string
  tone: ToastTone
  duration: number
}

type ConfirmOptions = {
  title: string
  message: string
  confirmText?: string
  cancelText?: string
  tone?: ToastTone
}

type ConfirmState = Required<ConfirmOptions>

let confirmResolver: ((accepted: boolean) => void) | null = null
let toastSeed = 1

export const useUiStore = defineStore('ui', {
  state: () => ({
    toasts: [] as ToastItem[],
    confirmState: null as ConfirmState | null
  }),
  actions: {
    toast(payload: string | Partial<Omit<ToastItem, 'id' | 'duration'>> & { message: string; duration?: number }) {
      const item: ToastItem =
        typeof payload === 'string'
          ? {
              id: toastSeed++,
              title: '',
              message: payload,
              tone: 'info',
              duration: 3200
            }
          : {
              id: toastSeed++,
              title: payload.title || '',
              message: payload.message,
              tone: payload.tone || 'info',
              duration: payload.duration ?? 3200
            }

      this.toasts = [...this.toasts, item]
      window.setTimeout(() => {
        this.dismissToast(item.id)
      }, item.duration)
      return item.id
    },
    dismissToast(id: number) {
      this.toasts = this.toasts.filter((item) => item.id !== id)
    },
    confirm(options: ConfirmOptions) {
      if (confirmResolver) {
        confirmResolver(false)
        confirmResolver = null
      }

      this.confirmState = {
        title: options.title,
        message: options.message,
        confirmText: options.confirmText || '确认',
        cancelText: options.cancelText || '取消',
        tone: options.tone || 'info'
      }

      return new Promise<boolean>((resolve) => {
        confirmResolver = resolve
      })
    },
    settleConfirm(accepted: boolean) {
      const resolver = confirmResolver
      confirmResolver = null
      this.confirmState = null
      resolver?.(accepted)
    }
  }
})
