import { beforeEach, describe, expect, it, vi } from 'vitest'
import { getReference, listAccounts, listFingerprintRecords, verifyFingerprint } from '../api'
const client = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))
vi.mock('@/api/client', () => ({ default: client }))
describe('model fingerprint API', () => {
  beforeEach(() => vi.clearAllMocks())
  it('pins an account and model, with sufficient timeout and cancellation', async () => {
    const signal = new AbortController().signal
    client.post.mockResolvedValue({ data: { id: 1 } })
    await verifyFingerprint(7, 'model', 'responses', signal)
    expect(client.post).toHaveBeenCalledWith('/admin/model-fingerprint/verify', { account_id: 7, model: 'model', api: 'responses' }, { timeout: 390000, signal })
  })
  it('passes history filters and reads cached reference without a download', async () => {
    client.get.mockResolvedValue({ data: { items: [], total: 0 } })
    await listFingerprintRecords({ page: 2, account_id: 7, model: 'model' })
    expect(client.get).toHaveBeenCalledWith('/admin/model-fingerprint/records', { params: { page: 2, page_size: 20, account_id: 7, model: 'model' } })
    await getReference()
    expect(client.get).toHaveBeenLastCalledWith('/admin/model-fingerprint/reference')
    expect(client.post).not.toHaveBeenCalled()
  })
  it('loads subsequent account pages rather than silently dropping accounts', async () => {
    client.get.mockResolvedValueOnce({ data: { items: [{ id: 1 }], total: 2 } }).mockResolvedValueOnce({ data: { items: [{ id: 2 }], total: 2 } })
    expect((await listAccounts()).map(a => a.id)).toEqual([1, 2])
    expect(client.get).toHaveBeenLastCalledWith('/admin/accounts', { params: { page: 2, page_size: 100, type: 'apikey', lite: true } })
  })
})
