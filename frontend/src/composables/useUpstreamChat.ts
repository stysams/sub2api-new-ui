import { ref } from 'vue'
import { buildApiUrl } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import type { UpstreamResource } from '@/types'

export interface ChatMessage {
  role: 'user' | 'assistant'
  content: string
}

export function useUpstreamChat() {
  const authStore = useAuthStore()
  const messages = ref<ChatMessage[]>([])
  const loading = ref(false)
  const model = ref('')
  const prompt = ref('')
  let abortController: AbortController | null = null

  function reset() {
    if (abortController) {
      abortController.abort()
      abortController = null
    }
    messages.value = []
    loading.value = false
    model.value = ''
    prompt.value = ''
  }

  function initForResource(resource: UpstreamResource) {
    reset()
    model.value = resource.models_snapshot?.[0] || ''
  }

  async function send(resource: UpstreamResource) {
    if (!prompt.value.trim() || !model.value || loading.value) return

    const userMessage = prompt.value.trim()
    prompt.value = ''
    messages.value.push({ role: 'user', content: userMessage })
    messages.value.push({ role: 'assistant', content: '' })
    const assistant = messages.value[messages.value.length - 1]
    loading.value = true

    abortController?.abort()
    const controller = new AbortController()
    abortController = controller

    try {
      const url = buildApiUrl(
        `/admin/upstreams/resources/${resource.id}/chat/completions`
      )
      const response = await fetch(url, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${authStore.token || ''}`
        },
        body: JSON.stringify({
          model: model.value,
          messages: messages.value.slice(0, -1),
          stream: true
        }),
        signal: controller.signal
      })

      if (!response.ok || !response.body) {
        const text = await response.text()
        throw new Error(text || `HTTP ${response.status}`)
      }

      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''

      while (true) {
        const { value, done } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split(/\r?\n/)
        buffer = lines.pop() || ''

        for (const line of lines) {
          if (!line.startsWith('data:')) continue
          const data = line.slice(5).trim()
          if (data === '[DONE]') continue
          try {
            const payload = JSON.parse(data)
            const delta =
              payload.choices?.[0]?.delta?.content ||
              payload.choices?.[0]?.text ||
              ''
            assistant.content += delta
          } catch {
            // Ignore incomplete SSE frames
          }
        }
      }
    } catch (error: unknown) {
      if (error instanceof DOMException && error.name === 'AbortError') {
        // 用户关闭对话框或离开页面，保留已接收内容，不视为错误
        return
      }
      const message =
        error instanceof Error ? error.message : 'Request failed'
      assistant.content = message
    } finally {
      if (abortController === controller) {
        abortController = null
        loading.value = false
      }
    }
  }

  return {
    messages,
    loading,
    model,
    prompt,
    reset,
    initForResource,
    send
  }
}
