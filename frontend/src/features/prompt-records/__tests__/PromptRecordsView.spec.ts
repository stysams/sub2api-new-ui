import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import PromptRecordsView from '../PromptRecordsView.vue'

const mocks = vi.hoisted(() => ({
  listPromptRecords: vi.fn(),
  getPromptRecord: vi.fn(),
  deletePromptRecord: vi.fn(),
		batchDeletePromptRecords: vi.fn(),
		deleteAllPromptRecords: vi.fn(),
	getPromptRecordingConfig: vi.fn(),
		updatePromptRecordingConfig: vi.fn(),
  previewPromptRecord: vi.fn(),
		copyToClipboard: vi.fn(),
	  searchUsers: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('../api', () => ({
  listPromptRecords: mocks.listPromptRecords,
  getPromptRecord: mocks.getPromptRecord,
  deletePromptRecord: mocks.deletePromptRecord,
	batchDeletePromptRecords: mocks.batchDeletePromptRecords,
	deleteAllPromptRecords: mocks.deleteAllPromptRecords,
	getPromptRecordingConfig: mocks.getPromptRecordingConfig,
	updatePromptRecordingConfig: mocks.updatePromptRecordingConfig,
  previewPromptRecord: mocks.previewPromptRecord,
}))

vi.mock('@/api/admin/usage', () => ({
  adminUsageAPI: { searchUsers: mocks.searchUsers },
  default: { searchUsers: mocks.searchUsers },
}))

vi.mock('@/stores/app', () => ({
	useAppStore: () => ({ showError: mocks.showError, showSuccess: mocks.showSuccess }),
}))

vi.mock('@/composables/useClipboard', () => ({
	useClipboard: () => ({ copyToClipboard: mocks.copyToClipboard }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
		useI18n: () => ({
			t: (key: string, params?: Record<string, unknown>) => {
				const rendered = key.replace(/\{(\w+)\}/g, (_, token) => String(params?.[token] ?? `{${token}}`))
				return params && 'count' in params ? `${rendered} ${String(params.count)}` : rendered
			},
		}),
  }
})

const summary = {
  id: 7,
	session_id: 'session-7',
  turn_no: 1,
  stage: 'http',
  user_id: 2,
  username: 'alice',
  user_email: 'alice@example.com',
  api_key_id: 3,
  api_key_name: 'primary',
  group_name: 'default',
  provider: 'openai',
  endpoint: '/v1/chat/completions',
  protocol: 'openai_chat',
  model: 'gpt-test',
  prompt_hash: 'a'.repeat(64),
  prompt_length: 28,
  message_count: 1,
  risk_status: 'pending',
  created_at: '2026-09-11T12:00:00Z',
}

const recommendedPolicy = {
  version: 1,
  keep: {
    user: true, assistant: true, tool_call: true, tool_result: true,
    system: false, developer: false, instructions: false,
    project: false, agent: false, environment: false, skills: false, reminder: false,
    thinking: false, reasoning_summary: false, tool_definition: false, metadata: false,
  },
}

describe('PromptRecordsView', () => {
  beforeEach(() => {
    Object.values(mocks).forEach((mock) => mock.mockReset())
    mocks.listPromptRecords.mockResolvedValue({
      items: [summary],
      page: 1,
      page_size: 20,
      total: 1,
      total_pages: 1,
      queue: {
        queue_length: 0,
        queue_capacity: 256,
        overflow_length: 0,
        overflow_capacity: 2048,
        worker_count: 4,
        dropped_total: 0,
        persist_failed_total: 0,
      },
    })
    mocks.getPromptRecord.mockResolvedValue({
      ...summary,
      prompt_text: 'PROMPT_DETAIL_ONLY_AFTER_CLICK',
      risk_result: '{}',
      response_text: 'RESPONSE_DETAIL_ONLY_AFTER_CLICK',
      response_length: 32,
      response_truncated: false,
      response_captured_at: '2026-09-11T12:00:02Z',
    })
    mocks.deletePromptRecord.mockResolvedValue(undefined)
		mocks.batchDeletePromptRecords.mockResolvedValue({ deleted: 1 })
		mocks.deleteAllPromptRecords.mockResolvedValue({ deleted: 983 })
		mocks.getPromptRecordingConfig.mockResolvedValue({
			enabled: true,
			headers_enabled: true,
			prompt_enabled: true,
			response_enabled: true,
			recording_policy: recommendedPolicy,
		})
		mocks.updatePromptRecordingConfig.mockResolvedValue({ enabled: false })
    mocks.previewPromptRecord.mockResolvedValue({
      content_document: {
        schema_version: 2,
        blocks: [{ category: 'user', text: 'READABLE_PREVIEW' }],
        filtered: { thinking: { count: 2, bytes: 321 }, encrypted_content: { count: 1, bytes: 123 } },
      },
      text: 'READABLE_PREVIEW',
      message_count: 1,
    })
		mocks.copyToClipboard.mockResolvedValue(true)
    mocks.searchUsers.mockResolvedValue([])
  })

  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('passes cancellation to requests and ignores stale failures', async () => {
    let rejectOld!: (reason: Error) => void
    mocks.listPromptRecords.mockImplementationOnce(() => new Promise((_, reject) => { rejectOld = reject }))
    const wrapper = mount(PromptRecordsView, { global: { stubs: { AppLayout: { template: '<div><slot /></div>' } } } })
    await flushPromises()
    const oldSignal = mocks.listPromptRecords.mock.calls[0][1] as AbortSignal
    expect(oldSignal.aborted).toBe(false)
    await wrapper.get('form').trigger('submit') // retention form does not reload records
    await wrapper.get('#prompt-record-model').setValue('latest')
    await wrapper.get('[data-test="prompt-record-filter-form"]').trigger('submit')
    await flushPromises()
    expect(oldSignal.aborted).toBe(true)
    rejectOld(new Error('late request failed'))
    await flushPromises()
    expect(mocks.showError).not.toHaveBeenCalledWith('admin.promptRecords.loadFailed')
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    const currentSignal = mocks.listPromptRecords.mock.calls[1][1] as AbortSignal
    expect(currentSignal).toBeInstanceOf(AbortSignal)
    wrapper.unmount()
  })

	it('uses cursor history and resets it when filters change', async () => {
		mocks.listPromptRecords.mockResolvedValue({ items: [summary], has_more: true, next_cursor: 'cursor-two', total: 983 })
    const wrapper = mount(PromptRecordsView, { global: { stubs: { AppLayout: { template: '<div><slot /></div>' } } } })
    await flushPromises()
		expect(mocks.listPromptRecords.mock.calls[0][0]).toMatchObject({ pagination: 'cursor', page: 1 })
		expect(wrapper.get('[data-test="record-total"]').text()).toContain('983')
    await wrapper.get('[data-test="record-next"]').trigger('click')
    await flushPromises()
    expect(mocks.listPromptRecords.mock.calls[1][0]).toMatchObject({ cursor: 'cursor-two', page: 2 })
    await wrapper.get('[data-test="record-previous"]').trigger('click')
    await flushPromises()
    expect(mocks.listPromptRecords.mock.calls[2][0].cursor).toBeUndefined()
    await wrapper.get('#prompt-record-model').setValue('new-model')
    await wrapper.get('[data-test="prompt-record-filter-form"]').trigger('submit')
    await flushPromises()
    expect(mocks.listPromptRecords.mock.lastCall?.[0]).toMatchObject({ page: 1, model: 'new-model', cursor: undefined })
    wrapper.unmount()
  })

  it('saves retention days and restores the saved value on failure', async () => {
    mocks.updatePromptRecordingConfig.mockResolvedValueOnce({ retention_days: 30 })
    const wrapper = mount(PromptRecordsView, { global: { stubs: { AppLayout: { template: '<div><slot /></div>' } } } })
    await flushPromises()
    const toolbar = wrapper.get('[data-test="prompt-recording-toolbar"]')
    expect(toolbar.find('[data-test="prompt-record-retention-form"]').exists()).toBe(true)
    expect(toolbar.element.textContent).toContain('admin.promptRecords.retentionDays')
    expect(toolbar.element.innerHTML.indexOf('prompt-recording-toggle')).toBeLessThan(
      toolbar.element.innerHTML.indexOf('prompt-record-retention'),
    )
    await wrapper.get('#prompt-record-retention').setValue(30)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(mocks.updatePromptRecordingConfig).toHaveBeenCalledWith({ retention_days: 30 })
    mocks.updatePromptRecordingConfig.mockRejectedValueOnce(new Error('save failed'))
    await wrapper.get('#prompt-record-retention').setValue(7)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect((wrapper.get('#prompt-record-retention').element as HTMLInputElement).value).toBe('30')
    wrapper.unmount()
  })

  it('explains missing WebSocket responses in details', async () => {
    mocks.getPromptRecord.mockResolvedValue({ ...summary, stage: 'subsequent_turn' })
    const wrapper = mount(PromptRecordsView, { attachTo: document.body, global: { stubs: { AppLayout: { template: '<div><slot /></div>' } } } })
    await flushPromises()
    await wrapper.get('[data-test="prompt-record-detail-7"]').trigger('click')
    await flushPromises()
    expect(document.body.textContent).toContain('admin.promptRecords.websocketResponseUnsupported')
    expect(document.body.textContent).not.toContain('admin.promptRecords.emptyResponse')
    wrapper.unmount()
  })

	it('loads full prompt text only after the row detail button is clicked', async () => {
    const wrapper = mount(PromptRecordsView, {
      attachTo: document.body,
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
        },
      },
	})

		await flushPromises()
    expect(mocks.listPromptRecords).toHaveBeenCalledOnce()
    expect(mocks.getPromptRecord).not.toHaveBeenCalled()
    expect(document.body.textContent).not.toContain('PROMPT_DETAIL_ONLY_AFTER_CLICK')
    expect(document.body.textContent).not.toContain('RESPONSE_DETAIL_ONLY_AFTER_CLICK')

    await wrapper.get('[data-test="prompt-record-detail-7"]').trigger('click')
    await flushPromises()

    expect(mocks.getPromptRecord).toHaveBeenCalledWith(7)
    expect(document.body.textContent).toContain('PROMPT_DETAIL_ONLY_AFTER_CLICK')
    expect(document.body.textContent).toContain('RESPONSE_DETAIL_ONLY_AFTER_CLICK')
		wrapper.unmount()
	})

	it('toggles the detail dialog into current-tab fullscreen mode', async () => {
		const wrapper = mount(PromptRecordsView, {
			attachTo: document.body,
			global: {
				stubs: {
					AppLayout: { template: '<div><slot /></div>' },
				},
			},
		})

		await flushPromises()
		await wrapper.get('[data-test="prompt-record-detail-7"]').trigger('click')
		await flushPromises()

		const getModal = () => document.body.querySelector<HTMLElement>('.modal-overlay')
		const getFullscreenButton = () => document.body.querySelector<HTMLButtonElement>('[data-test="prompt-record-detail-fullscreen"]')
		const fullscreenButton = getFullscreenButton()
		expect(fullscreenButton).not.toBeNull()
		expect(fullscreenButton?.getAttribute('aria-label')).toBe('admin.promptRecords.enterFullscreen')
		expect(getModal()?.classList.contains('modal-overlay-fullscreen')).toBe(false)

		fullscreenButton?.click()
		await flushPromises()
		expect(getModal()?.classList.contains('modal-overlay-fullscreen')).toBe(true)
		expect(getFullscreenButton()?.getAttribute('aria-label')).toBe('admin.promptRecords.exitFullscreen')

		document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
		await flushPromises()
		expect(getModal()?.classList.contains('modal-overlay-fullscreen')).toBe(false)
		expect(getFullscreenButton()).not.toBeNull()
		wrapper.unmount()
	})

	it('includes the API key name in the record search filters', async () => {
		const wrapper = mount(PromptRecordsView, { global: { stubs: { AppLayout: { template: '<div><slot /></div>' } } } })
		await flushPromises()
		await wrapper.get('#prompt-record-api-key').setValue('primary')
		await wrapper.get('[data-test="prompt-record-filter-form"]').trigger('submit')
		await flushPromises()
		expect(mocks.listPromptRecords.mock.lastCall?.[0]).toMatchObject({ api_key: 'primary', page: 1, cursor: undefined })
		wrapper.unmount()
	})

	it('updates the request and response recording switch', async () => {
		const wrapper = mount(PromptRecordsView, {
			global: {
				stubs: { AppLayout: { template: '<div><slot /></div>' } },
			},
		})
		await flushPromises()

		const toggle = wrapper.get('[data-test="prompt-recording-toggle"]')
		expect(toggle.attributes('aria-checked')).toBe('true')
		await toggle.trigger('click')
		await flushPromises()

		expect(mocks.updatePromptRecordingConfig).toHaveBeenCalledWith(false)
		expect(toggle.attributes('aria-checked')).toBe('false')
		expect(mocks.showSuccess).toHaveBeenCalledWith('admin.promptRecords.recordingDisabledSuccess')
		wrapper.unmount()
	})

  it('deletes one row only after confirmation', async () => {
    const wrapper = mount(PromptRecordsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          ConfirmDialog: {
            props: ['show'],
            emits: ['confirm', 'cancel'],
            template: '<button v-if="show" data-test="confirm-delete" @click="$emit(\'confirm\')">confirm</button>',
          },
        },
      },
    })
    await flushPromises()

    await wrapper.get('[data-test="prompt-record-delete-7"]').trigger('click')
    expect(mocks.deletePromptRecord).not.toHaveBeenCalled()
    await wrapper.get('[data-test="confirm-delete"]').trigger('click')
    await flushPromises()

    expect(mocks.deletePromptRecord).toHaveBeenCalledWith(7)
    expect(mocks.listPromptRecords).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })

  it('saves content switches independently and preserves state on failure', async () => {
		mocks.getPromptRecordingConfig.mockResolvedValue({ enabled: true, headers_enabled: true, prompt_enabled: true, response_enabled: true })
		mocks.updatePromptRecordingConfig.mockResolvedValueOnce({ enabled: true, headers_enabled: false, prompt_enabled: true, response_enabled: true })
    const wrapper = mount(PromptRecordsView, { global: { stubs: { AppLayout: { template: '<div><slot /></div>' } } } })
    await flushPromises()
		const headers = wrapper.get('[data-test="prompt-recording-headers_enabled"]')
		const prompt = wrapper.get('[data-test="prompt-recording-prompt_enabled"]')
		const response = wrapper.get('[data-test="prompt-recording-response_enabled"]')
    expect(headers.attributes('aria-checked')).toBe('true')
		expect(prompt.attributes('aria-checked')).toBe('true')
		expect(response.attributes('aria-checked')).toBe('true')
    await headers.trigger('click')
    await flushPromises()
    expect(mocks.updatePromptRecordingConfig).toHaveBeenCalledWith({ headers_enabled: false })
    expect(headers.attributes('aria-checked')).toBe('false')
		expect(prompt.attributes('aria-checked')).toBe('true')
		mocks.updatePromptRecordingConfig.mockResolvedValueOnce({ enabled: true, headers_enabled: false, prompt_enabled: true, response_enabled: false })
		await response.trigger('click')
		await flushPromises()
		expect(mocks.updatePromptRecordingConfig).toHaveBeenLastCalledWith({ response_enabled: false })
		expect(response.attributes('aria-checked')).toBe('false')
		mocks.updatePromptRecordingConfig.mockRejectedValueOnce(new Error('save failed'))
    await prompt.trigger('click')
    await flushPromises()
    expect(mocks.updatePromptRecordingConfig).toHaveBeenLastCalledWith({ prompt_enabled: false })
    expect(prompt.attributes('aria-checked')).toBe('true')
    expect(mocks.showError).toHaveBeenCalledWith('admin.promptRecords.recordingUpdateFailed')
    wrapper.unmount()
  })

  it('shows the complete request body and headers only in details', async () => {
		const requestBody = '{"tools":[{"name":"full-request-tool"}]}'
		mocks.getPromptRecord.mockResolvedValue({ ...summary, request_body: requestBody, request_headers: '{"X-Test":["saved-header"]}' })
    const wrapper = mount(PromptRecordsView, { attachTo: document.body, global: { stubs: { AppLayout: { template: '<div><slot /></div>' } } } })
    await flushPromises()
    expect(document.body.textContent).not.toContain('saved-header')
    await wrapper.get('[data-test="prompt-record-detail-7"]').trigger('click')
    await flushPromises()
		expect(document.body.textContent).toContain('full-request-tool')
		expect(document.body.textContent).toContain('saved-header')
		const copyButton = document.body.querySelector<HTMLButtonElement>('[data-test="prompt-record-copy-request-body"]')
		expect(copyButton).not.toBeNull()
		copyButton?.click()
		await flushPromises()
		expect(mocks.copyToClipboard).toHaveBeenCalledWith(requestBody, 'admin.promptRecords.requestBodyCopied')
    wrapper.unmount()
  })

  it('starts with policy settings collapsed and keeps an unsaved draft when folded', async () => {
    const wrapper = mount(PromptRecordsView, { global: { stubs: { AppLayout: { template: '<div><slot /></div>' } } } })
    await flushPromises()
    const policy = wrapper.get<HTMLDetailsElement>('[data-test="prompt-recording-policy"]')
    const toggle = wrapper.get('[data-test="prompt-policy-toggle"]')
    expect(policy.element.tagName).toBe('DETAILS')
    expect(policy.element.open).toBe(false)
    expect(policy.element.firstElementChild).toBe(toggle.element)
    expect(toggle.find('button').exists()).toBe(false)

    await toggle.trigger('click')
    expect(policy.element.open).toBe(true)
    await wrapper.get('[data-test="prompt-policy-keep-thinking"]').trigger('click')
    await toggle.trigger('click')
    expect(policy.element.open).toBe(false)
    expect(toggle.get('[data-test="prompt-policy-status"]').text()).toBe('admin.promptRecords.policyUnsaved')
    await toggle.trigger('click')
    expect(policy.element.open).toBe(true)
    expect(wrapper.get('[data-test="prompt-policy-keep-thinking"]').attributes('aria-checked')).toBe('true')
    expect(mocks.updatePromptRecordingConfig).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it.each([undefined, null])('shows all recommended category switches when no policy is returned (%s)', async (recordingPolicy) => {
    mocks.getPromptRecordingConfig.mockResolvedValue({ enabled: true, recording_policy: recordingPolicy })
    const wrapper = mount(PromptRecordsView, { global: { stubs: { AppLayout: { template: '<div><slot /></div>' } } } })
    await flushPromises()
    expect(wrapper.findAll('[data-test^="prompt-policy-keep-"]')).toHaveLength(16)
    for (const [category, keep] of Object.entries(recommendedPolicy.keep)) {
      expect(wrapper.get(`[data-test="prompt-policy-keep-${category}"]`).attributes('aria-checked')).toBe(String(keep))
    }
    expect(wrapper.find('[data-test^="prompt-recording-filter_"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="prompt-recording-preset-options"]').exists()).toBe(false)
    expect(wrapper.get('[data-test="prompt-policy-recommended"]').text()).toBe('admin.promptRecords.resetRecommendedPolicy')
    expect(wrapper.get('[data-test="prompt-policy-save"]').attributes('disabled')).toBeDefined()
    expect(mocks.updatePromptRecordingConfig).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('restores recommended values only in the draft and saves the complete policy', async () => {
    const savedPolicy = { ...recommendedPolicy, keep: { ...recommendedPolicy.keep, system: true, thinking: true } }
    mocks.getPromptRecordingConfig.mockResolvedValue({ enabled: true, recording_policy: savedPolicy })
    const wrapper = mount(PromptRecordsView, { global: { stubs: { AppLayout: { template: '<div><slot /></div>' } } } })
    await flushPromises()
    expect(wrapper.get('[data-test="prompt-policy-keep-system"]').attributes('aria-checked')).toBe('true')
    expect(wrapper.get('[data-test="prompt-policy-keep-thinking"]').attributes('aria-checked')).toBe('true')

    await wrapper.get('[data-test="prompt-policy-recommended"]').trigger('click')
    expect(mocks.updatePromptRecordingConfig).not.toHaveBeenCalled()
    expect(wrapper.get('[data-test="prompt-policy-keep-system"]').attributes('aria-checked')).toBe('false')
    expect(wrapper.get('[data-test="prompt-policy-keep-thinking"]').attributes('aria-checked')).toBe('false')
    expect(wrapper.get('[data-test="prompt-policy-save"]').attributes('disabled')).toBeUndefined()
    await wrapper.get('[data-test="prompt-policy-discard"]').trigger('click')
    expect(wrapper.get('[data-test="prompt-policy-keep-system"]').attributes('aria-checked')).toBe('true')
    expect(wrapper.get('[data-test="prompt-policy-keep-thinking"]').attributes('aria-checked')).toBe('true')

    await wrapper.get('[data-test="prompt-policy-recommended"]').trigger('click')
    mocks.updatePromptRecordingConfig.mockResolvedValueOnce({ recording_policy: recommendedPolicy })
    await wrapper.get('[data-test="prompt-policy-save"]').trigger('click')
    await flushPromises()
    expect(mocks.updatePromptRecordingConfig).toHaveBeenCalledTimes(1)
    expect(mocks.updatePromptRecordingConfig).toHaveBeenCalledWith({ recording_policy: recommendedPolicy })
    expect(wrapper.get('[data-test="prompt-policy-save"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-test="prompt-policy-status"]').text()).toBe('admin.promptRecords.policyApplied')
    wrapper.unmount()
  })

  it('preserves policy drafts across other setting updates and save failures', async () => {
    mocks.getPromptRecordingConfig.mockResolvedValue({ enabled: true, recording_policy: recommendedPolicy })
    const wrapper = mount(PromptRecordsView, { global: { stubs: { AppLayout: { template: '<div><slot /></div>' } } } })
    await flushPromises()
    await wrapper.get('[data-test="prompt-policy-keep-system"]').trigger('click')
    expect(mocks.updatePromptRecordingConfig).not.toHaveBeenCalled()
    mocks.updatePromptRecordingConfig.mockResolvedValueOnce({ enabled: true, headers_enabled: false, recording_policy: recommendedPolicy })
    await wrapper.get('[data-test="prompt-recording-headers_enabled"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-test="prompt-policy-keep-system"]').attributes('aria-checked')).toBe('true')

    mocks.updatePromptRecordingConfig.mockRejectedValueOnce(new Error('save failed'))
    await wrapper.get('[data-test="prompt-policy-save"]').trigger('click')
    await flushPromises()
    expect(mocks.updatePromptRecordingConfig).toHaveBeenLastCalledWith({ recording_policy: { ...recommendedPolicy, keep: { ...recommendedPolicy.keep, system: true } } })
    expect(wrapper.get('[data-test="prompt-policy-keep-system"]').attributes('aria-checked')).toBe('true')
    expect(wrapper.get('[data-test="prompt-policy-save"]').attributes('disabled')).toBeUndefined()
    expect(mocks.showError).toHaveBeenCalledWith('admin.promptRecords.recordingUpdateFailed')
    await wrapper.get('[data-test="prompt-policy-discard"]').trigger('click')
    expect(wrapper.get('[data-test="prompt-policy-keep-system"]').attributes('aria-checked')).toBe('false')
    wrapper.unmount()
  })

  it('previews the draft without saving and clears stale results when inputs change', async () => {
    const wrapper = mount(PromptRecordsView, { global: { stubs: { AppLayout: { template: '<div><slot /></div>' } } } })
    await flushPromises()
    const input = wrapper.get('#prompt-preview-body')
    await input.setValue('invalid')
    await wrapper.get('[data-test="prompt-policy-preview-form"]').trigger('submit')
    expect(mocks.previewPromptRecord).not.toHaveBeenCalled()
    expect(wrapper.get('[data-test="prompt-policy-preview-error"]').text()).toBe('admin.promptRecords.previewInvalidJSON')

    const body = { input: 'READABLE_PREVIEW' }
    await input.setValue(JSON.stringify(body))
    await wrapper.get('[data-test="prompt-policy-preview-form"]').trigger('submit')
    await flushPromises()
    expect(mocks.previewPromptRecord).toHaveBeenCalledWith({ protocol: 'openai_responses', body, recording_policy: recommendedPolicy, max_messages: 30 }, expect.any(AbortSignal))
    expect(mocks.updatePromptRecordingConfig).not.toHaveBeenCalled()
    expect(wrapper.get('[data-test="prompt-policy-preview-result"]').text()).toContain('READABLE_PREVIEW')
    expect(wrapper.get('[data-test="prompt-policy-preview-filtered"]').text()).toContain('321')
    expect(wrapper.get('[data-test="prompt-policy-preview-filtered"]').text()).toContain('123')
    await wrapper.get('[data-test="prompt-policy-keep-thinking"]').trigger('click')
    expect(wrapper.find('[data-test="prompt-policy-preview-result"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('ignores an outdated preview that finishes after the request source changes', async () => {
    let resolvePreview!: (value: unknown) => void
    mocks.previewPromptRecord.mockImplementationOnce(() => new Promise((resolve) => { resolvePreview = resolve }))
    const wrapper = mount(PromptRecordsView, { global: { stubs: { AppLayout: { template: '<div><slot /></div>' } } } })
    await flushPromises()
    await wrapper.get('#prompt-preview-body').setValue('{"input":"old"}')
    await wrapper.get('[data-test="prompt-policy-preview-form"]').trigger('submit')
    const signal = mocks.previewPromptRecord.mock.calls[0][1] as AbortSignal
    await wrapper.get('#prompt-preview-body').setValue('{"input":"new"}')
    expect(signal.aborted).toBe(true)
    resolvePreview({ content_document: { schema_version: 2, blocks: [{ category: 'user', text: 'OLD_RESULT' }], filtered: {} }, text: 'OLD_RESULT', message_count: 1 })
    await flushPromises()
    expect(wrapper.find('[data-test="prompt-policy-preview-result"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('OLD_RESULT')
    wrapper.unmount()
  })

  it('shows normalized request and response blocks including structured tool values', async () => {
    mocks.getPromptRecord.mockResolvedValue({
      ...summary,
      content_document: JSON.stringify({ schema_version: 2, blocks: [{ category: 'user', text: 'VISIBLE_REQUEST' }, { category: 'tool_result', data: { valid: false, count: 0, next: null } }], filtered: { thinking: { count: 1, bytes: 12 } } }),
      response_document: JSON.stringify({ schema_version: 2, blocks: [{ category: 'tool_call', name: 'read_file', data: { path: 'main.go' } }], filtered: {} }),
      response_text: '',
      response_captured_at: '2026-09-11T12:00:02Z',
    })
    const wrapper = mount(PromptRecordsView, { attachTo: document.body, global: { stubs: { AppLayout: { template: '<div><slot /></div>' } } } })
    await flushPromises()
    await wrapper.get('[data-test="prompt-record-detail-7"]').trigger('click')
    await flushPromises()
    const request = document.body.querySelector('[data-test="prompt-record-content-document"]')
    expect(request?.textContent).toContain('VISIBLE_REQUEST')
    expect(request?.textContent).toContain('"valid": false')
    expect(request?.textContent).toContain('"count": 0')
    expect(request?.textContent).toContain('"next": null')
    const response = document.body.querySelector('[data-test="prompt-record-response-document"]')
    expect(response?.textContent).toContain('read_file')
    expect(response?.textContent).toContain('main.go')
    wrapper.unmount()
  })

	it('batch deletes selected rows', async () => {
    const wrapper = mount(PromptRecordsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          ConfirmDialog: {
            props: ['show'],
            emits: ['confirm', 'cancel'],
            template: '<button v-if="show" data-test="confirm-delete" @click="$emit(\'confirm\')">confirm</button>',
          },
        },
      },
    })
    await flushPromises()

    await wrapper.get('[data-test="select-row"]').setValue(true)
    await wrapper.get('[data-test="prompt-record-batch-delete"]').trigger('click')
    await wrapper.get('[data-test="confirm-delete"]').trigger('click')
    await flushPromises()

		expect(mocks.batchDeletePromptRecords).toHaveBeenCalledWith([7])
		wrapper.unmount()
	})

	it('deletes every prompt record only after confirmation', async () => {
		mocks.listPromptRecords
			.mockResolvedValueOnce({ items: [summary], total: 983, has_more: true, next_cursor: 'next' })
			.mockResolvedValueOnce({ items: [], total: 0, has_more: false })
		const wrapper = mount(PromptRecordsView, {
			global: {
				stubs: {
					AppLayout: { template: '<div><slot /></div>' },
					ConfirmDialog: {
						props: ['show', 'title', 'message'], emits: ['confirm', 'cancel'],
						template: '<button v-if="show" data-test="confirm-delete" @click="$emit(\'confirm\')">{{ title }} {{ message }}</button>',
					},
				},
			},
		})
		await flushPromises()
		await wrapper.get('[data-test="prompt-record-delete-all"]').trigger('click')
		expect(mocks.deleteAllPromptRecords).not.toHaveBeenCalled()
		expect(wrapper.get('[data-test="confirm-delete"]').text()).toContain('983')
		await wrapper.get('[data-test="confirm-delete"]').trigger('click')
		await flushPromises()
		expect(mocks.deleteAllPromptRecords).toHaveBeenCalledOnce()
		expect(mocks.listPromptRecords.mock.lastCall?.[0]).toMatchObject({ page: 1, cursor: undefined })
		wrapper.unmount()
	})

	it('does not render the record processing status panel', async () => {
		const wrapper = mount(PromptRecordsView, { global: { stubs: { AppLayout: { template: '<div><slot /></div>' } } } })
		await flushPromises()
		expect(wrapper.find('[data-test="record-queue-stats"]').exists()).toBe(false)
		wrapper.unmount()
	})
})
