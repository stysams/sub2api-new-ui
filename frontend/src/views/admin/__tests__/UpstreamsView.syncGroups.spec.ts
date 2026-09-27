import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import UpstreamsView from '@/views/admin/UpstreamsView.vue'
import type { AdminGroup, Upstream, UpstreamResource } from '@/types'

const { listPaginated, listResources, listUpstreamGroups, getAllGroups, syncToAccount } = vi.hoisted(() => ({
  listPaginated: vi.fn(),
  listResources: vi.fn(),
  listUpstreamGroups: vi.fn(),
  getAllGroups: vi.fn(),
  syncToAccount: vi.fn()
}))

vi.mock('@/api', () => ({
  adminAPI: {
    upstreams: {
      listPaginated,
      resources: listResources,
      groups: listUpstreamGroups,
      getBalanceNotifySettings: vi.fn().mockResolvedValue({ enabled: false, threshold: 0, emails: [] }),
      syncToAccount
    },
    groups: { getAll: getAllGroups }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess: vi.fn(), showWarning: vi.fn(), showError: vi.fn() })
}))

vi.mock('@/composables/useUpstreamChat', () => ({
  useUpstreamChat: () => ({ reset: vi.fn(), initForResource: vi.fn() })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const AppLayoutStub = defineComponent({ template: '<main><slot /></main>' })
const BaseDialogStub = defineComponent({
  props: ['show', 'title'],
  template: '<section v-if="show" role="dialog"><h2>{{ title }}</h2><slot /><slot name="footer" /></section>'
})

const upstream = {
  id: 3,
  name: 'Relay',
  kind: 'sub2api',
  base_url: 'https://relay.example.com',
  balance_snapshot: {},
  enabled: true,
  resource_count: 1
} as Upstream

const resource = {
  id: 7,
  upstream_id: 3,
  name: 'Relay key',
  models_snapshot: [],
  key_masked: 'sk-***',
  enabled: true
} as UpstreamResource

function group(id: number, name: string, platform: AdminGroup['platform'], requireOAuthOnly = false): AdminGroup {
  return { id, name, platform, require_oauth_only: requireOAuthOnly, status: 'active' } as AdminGroup
}

describe('UpstreamsView platform group sync', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    listPaginated.mockResolvedValue({ items: [upstream], total: 1, pages: 1 })
    listResources.mockResolvedValue([resource])
    listUpstreamGroups.mockResolvedValue([])
    getAllGroups.mockResolvedValue([
      group(1, 'OpenAI primary', 'openai'),
      group(2, 'Anthropic primary', 'anthropic'),
      group(3, 'Composite', 'composite'),
      group(4, 'Gemini OAuth', 'gemini', true),
      { ...group(5, 'Inactive OpenAI', 'openai'), status: 'inactive' }
    ])
    syncToAccount.mockResolvedValue({ created: 2, updated: 0, skipped: 0, failed: 0, items: [] })
  })

  it('groups eligible checkboxes by platform and submits selections across platforms', async () => {
    const wrapper = mount(UpstreamsView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          BaseDialog: BaseDialogStub,
          ConfirmDialog: true,
          Icon: true
        }
      }
    })
    await flushPromises()
    await wrapper.get('button[aria-label="admin.upstreams.expand Relay"]').trigger('click')
    await flushPromises()
    await wrapper.get('button[aria-label="admin.upstreams.sync"]').trigger('click')
    await flushPromises()

    const dialog = wrapper.get('[role="dialog"]')
    expect(dialog.findAll('legend').map(item => item.text())).toEqual(['Anthropic', 'OpenAI'])
    expect(dialog.findAll('input[type="checkbox"]')).toHaveLength(2)
    await dialog.get('input[value="1"]').setValue(true)
    await dialog.get('input[value="2"]').setValue(true)
    await dialog.get('button.btn-primary').trigger('click')
    await flushPromises()

    expect(syncToAccount).toHaveBeenCalledWith(7, [1, 2])
    wrapper.unmount()
  })
})
