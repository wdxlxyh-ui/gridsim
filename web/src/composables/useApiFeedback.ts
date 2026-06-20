import { ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { ElMessageBoxOptions } from 'element-plus'

export function formatApiError(err: unknown, fallback = '操作失败'): string {
  const e = err as any
  const body = e?.response?.data
  if (body?.error?.message) {
    let msg = body.error.message
    if (body.error.hint) msg += `。${body.error.hint}`
    if (body.error.code) msg = `[${body.error.code}] ${msg}`
    return msg
  }
  if (body?.error) return String(body.error)
  if (e?.code === 'ECONNABORTED') return '请求超时，请检查网络连接'
  if (e?.code === 'ERR_NETWORK') return '网络连接失败，请检查后端服务'
  if (e?.message) return e.message
  return fallback
}

/**
 * Show API success feedback (toast + optional text in toast).
 */
export function apiSuccess(msg: string) {
  ElMessage.success(msg)
}

export function apiWarning(msg: string) {
  ElMessage.warning(msg)
}

export function apiError(err: unknown, fallback = '操作失败') {
  ElMessage.error(formatApiError(err, fallback))
}

export async function apiConfirm(
  message: string,
  title = '确认',
  options?: Partial<ElMessageBoxOptions>
): Promise<boolean> {
  try {
    await ElMessageBox.confirm(message, title, {
      type: 'warning',
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      ...options,
    })
    return true
  } catch {
    return false
  }
}

export function useApiAction<TArgs extends any[], TResult>(
  action: (...args: TArgs) => Promise<TResult>,
  feedback?: {
    success?: string | ((result: TResult) => string)
    error?: string
    confirm?: string
  }
) {
  const loading = ref(false)

  async function execute(...args: TArgs): Promise<TResult | undefined> {
    if (feedback?.confirm) {
      const ok = await apiConfirm(feedback.confirm)
      if (!ok) return undefined
    }

    loading.value = true
    try {
      const result = await action(...args)
      if (feedback?.success) {
        const msg = typeof feedback.success === 'function'
          ? feedback.success(result)
          : feedback.success
        apiSuccess(msg)
      }
      return result
    } catch (err) {
      apiError(err, feedback?.error)
      return undefined
    } finally {
      loading.value = false
    }
  }

  return { execute, loading }
}
