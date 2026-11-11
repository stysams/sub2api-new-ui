import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ModelFingerprintView from '../ModelFingerprintView.vue'
const api = vi.hoisted(() => ({ getReference: vi.fn(), listAccounts: vi.fn(), listFingerprintRecords: vi.fn(), refreshReference: vi.fn(), verifyFingerprint: vi.fn() }))
vi.mock('../api', () => api)
vi.mock('vue-i18n', async importOriginal => ({ ...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
const create = () => mount(ModelFingerprintView, { global: { stubs: { AppLayout: { template: '<div><slot /></div>' }, Pagination: true } } })
const findButton = (wrapper: ReturnType<typeof create>, key: string) => wrapper.findAll('button').find(b => b.text().includes(key))!
describe('fingerprint workflow', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    api.getReference.mockResolvedValue({ available: false })
    api.listAccounts.mockResolvedValue([{ id: 7, name: 'upstream', platform: 'openai', type: 'apikey' }])
    api.listFingerprintRecords.mockResolvedValue({ items: [], total: 0 })
  })
  it('opens in the app layout, reads cached data and requires an explicit bank download', async () => {
    const w = create(); await flushPromises()
    expect(api.refreshReference).not.toHaveBeenCalled()
    expect(api.verifyFingerprint).not.toHaveBeenCalled()
    expect(findButton(w, 'admin.modelFingerprint.verify').attributes('disabled')).toBeDefined()
    api.refreshReference.mockResolvedValue({ available: true, built_at: 'fixture-v1', sha256: 'abc' })
    await findButton(w, 'admin.modelFingerprint.download').trigger('click'); await flushPromises()
    expect(api.refreshReference).toHaveBeenCalledOnce()
    expect(w.text()).toContain('fixture-v1')
    w.unmount()
  })
  it('shows sampling evidence and ranked candidates from the selected account', async () => {
    api.getReference.mockResolvedValue({ available: true })
    api.verifyFingerprint.mockResolvedValue({ id: 1, account_id: 7, account_name: 'upstream', model: 'requested', upstream_model: 'mapped', endpoint: '/v1/responses', status: 'scored', samples: [{ prompt: 'numbers', count: 300, expected_count: 300, status_code: 200, latency_ms: 5, answer: '17, 200' }], analysis: { used_outputs: 3, bank_version: 'fixture-v1', reason: 'reference_closed_set', results: [{ model: 'candidate-a', display_name: 'Candidate A', score: 0.123, probability: 0.5 }] } })
    const w = create(); await flushPromises()
    await w.find('form select').setValue('7'); await w.find('form input').setValue('requested'); await w.find('form').trigger('submit'); await flushPromises()
    expect(api.verifyFingerprint).toHaveBeenCalledWith(7, 'requested', 'auto', expect.any(AbortSignal))
    expect(w.text()).toContain('Candidate A'); expect(w.text()).toContain('50.0%'); expect(w.text()).toContain('mapped'); expect(w.text()).toContain('300/300')
    w.unmount()
  })
  it('surfaces failed downloads and keeps verification disabled', async () => {
    api.refreshReference.mockRejectedValue(new Error('reference HTTP 403'))
    const w = create(); await flushPromises()
    await findButton(w, 'admin.modelFingerprint.download').trigger('click'); await flushPromises()
    expect(w.find('[role="alert"]').text()).toContain('reference HTTP 403')
    expect(findButton(w, 'admin.modelFingerprint.verify').attributes('disabled')).toBeDefined()
    w.unmount()
  })
})
