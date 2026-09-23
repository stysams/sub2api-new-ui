import { afterEach, describe, expect, it, vi } from 'vitest'
import { useUpstreamChat } from '@/composables/useUpstreamChat'
import type { UpstreamResource } from '@/types'

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ token: 'test-token' })
}))

const resource = {
  id: 1,
  models_snapshot: ['test-model']
} as UpstreamResource

describe('useUpstreamChat', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('does not clear a replacement request loading state when an aborted request settles', async () => {
    let resolveReplacement!: (response: Response) => void
    const fetchMock = vi
      .fn()
      .mockImplementationOnce((_url: string, init: RequestInit) =>
        new Promise<Response>((_resolve, reject) => {
          init.signal?.addEventListener('abort', () => {
            reject(new DOMException('Aborted', 'AbortError'))
          })
        })
      )
      .mockImplementationOnce(
        () => new Promise<Response>((resolve) => {
          resolveReplacement = resolve
        })
      )
    vi.stubGlobal('fetch', fetchMock)

    const chat = useUpstreamChat()
    chat.initForResource(resource)
    chat.prompt.value = 'first request'
    const firstRequest = chat.send(resource)

    chat.reset()
    chat.initForResource(resource)
    chat.prompt.value = 'replacement request'
    const replacementRequest = chat.send(resource)

    await firstRequest
    expect(chat.loading.value).toBe(true)

    resolveReplacement(new Response('data: [DONE]\n\n'))
    await replacementRequest
    expect(chat.loading.value).toBe(false)
  })
})
