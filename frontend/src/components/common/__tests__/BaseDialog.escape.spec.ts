import { afterEach, describe, expect, it } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import BaseDialog from '../BaseDialog.vue'

enableAutoUnmount(afterEach)
const pressEscape = () => document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))

describe('stacked dialog Escape handling', () => {
  it('exits fullscreen on the top dialog before allowing the parent to handle Escape', async () => {
    const parent = mount(BaseDialog, { props: { show: true, title: 'Parent', fullscreen: true } })
    const child = mount(BaseDialog, { props: { show: true, title: 'Child', fullscreen: true } })
    pressEscape()
    expect(child.emitted('exit-fullscreen')).toHaveLength(1)
    expect(child.emitted('close')).toBeUndefined()
    expect(parent.emitted('exit-fullscreen')).toBeUndefined()
    await child.setProps({ fullscreen: false })
    pressEscape()
    expect(child.emitted('close')).toHaveLength(1)
    await child.setProps({ show: false })
    pressEscape()
    expect(parent.emitted('exit-fullscreen')).toHaveLength(1)
    expect(parent.emitted('close')).toBeUndefined()
  })

  it('closes only the most recently opened dialog, then allows the parent to close', async () => {
    const parent = mount(BaseDialog, { props: { show: true, title: 'Parent' } })
    const child = mount(BaseDialog, { props: { show: true, title: 'Child' } })
    pressEscape()
    expect(child.emitted('close')).toHaveLength(1)
    expect(parent.emitted('close')).toBeUndefined()
    await child.setProps({ show: false })
    pressEscape()
    expect(parent.emitted('close')).toHaveLength(1)
  })

  it('does not dismiss a parent behind a child that disallows Escape', () => {
    const parent = mount(BaseDialog, { props: { show: true, title: 'Parent' } })
    const child = mount(BaseDialog, { props: { show: true, title: 'Child', closeOnEscape: false } })
    pressEscape()
    expect(child.emitted('close')).toBeUndefined()
    expect(parent.emitted('close')).toBeUndefined()
    child.unmount()
    pressEscape()
    expect(parent.emitted('close')).toHaveLength(1)
  })
})
