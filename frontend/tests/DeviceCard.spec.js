import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'
import DeviceCard from '@/components/DeviceCard.vue'

const device = {
  id: 'd1',
  name: 'Harbor Hauler',
  providerName: 'Truck 04',
  renamed: true,
  make: 'Freightliner',
  model: 'M2 106',
  online: true,
  driveStatus: 'driving',
  groups: ['Logistics'],
  position: {
    lat: 32.715736,
    lng: -117.161087,
    speed: 96.56,
    speedUnit: 'km/h',
    heading: 90,
    valid: true,
    recordedAt: new Date().toISOString(),
  },
  preferences: { hidden: false, pinned: false, markerIcon: 'truck', markerColor: '#2E7D32', notes: '' },
}

function mountCard(overrides = {}) {
  return mount(DeviceCard, {
    props: { device: { ...device, ...overrides }, speedUnit: 'mph' },
  })
}

describe('DeviceCard', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('shows the custom name and keeps the provider name visible', () => {
    const text = mountCard().text()
    expect(text).toContain('Harbor Hauler')
    expect(text).toContain('Truck 04')
  })

  it('converts the speed into the unit the user picked', () => {
    expect(mountCard().text()).toContain('60 mph')
  })

  it('says so plainly when there is no GPS fix', () => {
    const wrapper = mountCard({ position: { ...device.position, valid: false } })
    expect(wrapper.text()).toContain('No GPS fix')
  })

  it('marks offline and hidden vehicles', () => {
    const wrapper = mountCard({ online: false, preferences: { ...device.preferences, hidden: true } })
    expect(wrapper.text()).toContain('Offline')
    expect(wrapper.text()).toContain('Hidden')
  })

  it('emits the actions the list needs, with the device attached', async () => {
    const wrapper = mountCard()

    await wrapper.get('[aria-label="Select Harbor Hauler"]').trigger('click')
    expect(wrapper.emitted('select')[0]).toEqual(['d1'])

    await wrapper.get('[aria-label="Pin Harbor Hauler"]').trigger('click')
    expect(wrapper.emitted('toggle-pin')).toHaveLength(1)

    await wrapper.get('[aria-label="Hide Harbor Hauler"]').trigger('click')
    expect(wrapper.emitted('toggle-hidden')).toHaveLength(1)

    await wrapper.get('[aria-label="Customise Harbor Hauler"]').trigger('click')
    expect(wrapper.emitted('edit')).toHaveLength(1)
  })

  it('flips the pin control’s label once the vehicle is pinned', () => {
    const wrapper = mountCard({ preferences: { ...device.preferences, pinned: true } })
    expect(wrapper.find('[aria-label="Unpin Harbor Hauler"]').exists()).toBe(true)
  })

  it('renders a custom uploaded marker instead of the 3D model', () => {
    const wrapper = mountCard({
      preferences: { ...device.preferences, markerIcon: 'custom', customIconUrl: '/api/v1/icons/abc' },
    })
    expect(wrapper.get('img').attributes('src')).toBe('/api/v1/icons/abc')
  })
})
