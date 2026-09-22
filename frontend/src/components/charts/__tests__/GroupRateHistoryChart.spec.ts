import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import GroupRateHistoryChart from '../GroupRateHistoryChart.vue'
import type { GroupRateBoardItem } from '@/api/groupRates'

vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key, locale: ref('en') }),
}))
vi.mock('vue-chartjs', () => ({
  Line: { props: ['data', 'options'], template: '<div class="chart-data">{{ JSON.stringify(data) }}</div>' },
}))

const usage = { requests: 0, input_tokens: 0, output_tokens: 0, cache_creation_tokens: 0, cache_read_tokens: 0, total_tokens: 0 }
const group = (id: number, points: GroupRateBoardItem['rate_trend']): GroupRateBoardItem => ({
  id, name: `Group ${id}`, platform: 'openai', subscription_type: 'standard', rate_multiplier: 1,
  peak_multiplier: 1, effective_multiplier: 1, dynamic_rate: { enabled: true, min: .5, max: 2 },
  last_hour: usage, last_24_hours: usage, trend: [], rate_trend: points,
})

describe('GroupRateHistoryChart', () => {
  it('renders separate stepped series and carries each published rate forward', () => {
    const wrapper = mount(GroupRateHistoryChart, { props: { groups: [
      group(1, [
        { at: '2026-09-22T08:00:00Z', rate_multiplier: 1 },
        { at: '2026-09-22T08:10:00Z', rate_multiplier: 1.25 },
      ]),
      group(2, [{ at: '2026-09-22T08:05:00Z', rate_multiplier: .8 }]),
    ] } })
    const data = JSON.parse(wrapper.get('.chart-data').text())
    expect(data.datasets[0].data).toEqual([1, 1, 1.25])
    expect(data.datasets[1].data).toEqual([null, .8, .8])
    expect(data.datasets[0].stepped).toBe('after')
  })

  it('shows an empty state until the first history sample exists', () => {
    const wrapper = mount(GroupRateHistoryChart, { props: { groups: [group(1, [])] } })
    expect(wrapper.text()).toContain('groupRates.noRateHistory')
    expect(wrapper.find('.chart-data').exists()).toBe(false)
  })
})
