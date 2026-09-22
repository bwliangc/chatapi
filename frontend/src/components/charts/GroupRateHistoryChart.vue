<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Line } from 'vue-chartjs'
import { Chart as ChartJS, CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend, type ChartOptions } from 'chart.js'
import type { GroupRateBoardItem } from '@/api/groupRates'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend)
const props = defineProps<{ groups: GroupRateBoardItem[] }>()
const { t, locale } = useI18n()
const colors = ['#0f766e', '#2563eb', '#d97706', '#dc2626', '#7c3aed', '#0891b2', '#65a30d', '#db2777']
const groupsWithHistory = computed(() => props.groups.filter(group => group.rate_trend.length > 0))
const timestamps = computed(() => [...new Set(groupsWithHistory.value.flatMap(group => group.rate_trend.map(point => point.at)))].sort())
const chartData = computed(() => ({
  labels: timestamps.value.map(at => new Date(at).toLocaleTimeString(locale.value, { hour: '2-digit', minute: '2-digit' })),
  datasets: groupsWithHistory.value.map((group, index) => {
    const points = [...group.rate_trend].sort((a, b) => a.at.localeCompare(b.at))
    let pointIndex = 0
    let current: number | null = null
    const data = timestamps.value.map(at => {
      while (pointIndex < points.length && points[pointIndex].at <= at) {
        current = points[pointIndex].rate_multiplier
        pointIndex++
      }
      return current
    })
    return {
      label: group.name,
      data,
      borderColor: colors[index % colors.length],
      backgroundColor: colors[index % colors.length],
      borderWidth: 2,
      stepped: 'after' as const,
      spanGaps: false,
      pointRadius: points.length < 3 ? 2 : 0,
      pointHoverRadius: 4,
    }
  }),
}))
const options = computed<ChartOptions<'line'>>(() => ({
  responsive: true,
  maintainAspectRatio: false,
  animation: false,
  interaction: { mode: 'index', intersect: false },
  plugins: {
    legend: {
      display: groupsWithHistory.value.length > 1,
      position: 'bottom',
      labels: { color: '#64748b', boxWidth: 10, boxHeight: 2, usePointStyle: true, pointStyle: 'line' },
    },
    tooltip: {
      callbacks: {
        title: items => {
          const at = timestamps.value[items[0]?.dataIndex ?? 0]
          return at ? new Date(at).toLocaleString(locale.value, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }) : ''
        },
        label: item => `${item.dataset.label}: ${Number(item.raw).toLocaleString(locale.value, { maximumFractionDigits: 4 })}×`,
      },
    },
  },
  scales: {
    x: { grid: { display: false }, ticks: { color: '#94a3b8', maxTicksLimit: 7, maxRotation: 0 }, border: { display: false } },
    y: {
      grid: { color: 'rgba(148,163,184,0.12)' },
      ticks: { color: '#94a3b8', maxTicksLimit: 5, callback: value => `${Number(value).toLocaleString(locale.value, { maximumFractionDigits: 4 })}×` },
      border: { display: false },
    },
  },
}))
</script>

<template>
  <div v-if="groupsWithHistory.length" class="h-56 sm:h-64">
    <Line :data="chartData" :options="options" :aria-label="t('groupRates.rateTrendTitle')" role="img" />
  </div>
  <div v-else class="flex h-56 items-center justify-center text-sm text-gray-400 sm:h-64">
    {{ t('groupRates.noRateHistory') }}
  </div>
</template>
