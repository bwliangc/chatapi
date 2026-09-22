<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Line } from 'vue-chartjs'
import { Chart as ChartJS, CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Filler, type ChartOptions } from 'chart.js'
import type { GroupRateTrendPoint } from '@/api/groupRates'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Filler)
const props = defineProps<{ points: GroupRateTrendPoint[]; metric: 'tokens' | 'requests' }>()
const { t, locale } = useI18n()
const compact = (value: number) => new Intl.NumberFormat(locale.value, { notation: 'compact', maximumFractionDigits: 1 }).format(value)
const chartData = computed(() => ({
  labels: props.points.map(point => new Date(point.at).toLocaleTimeString(locale.value, { hour: '2-digit', minute: '2-digit' })),
  datasets: [{
    label: props.metric === 'tokens' ? 'Tokens' : t('groupRates.requests'),
    data: props.points.map(point => point[props.metric]),
    borderColor: '#14b8a6', backgroundColor: 'rgba(20,184,166,0.09)',
    fill: true, borderWidth: 2, tension: 0.25, pointRadius: 0, pointHoverRadius: 4,
  }],
}))
const options = computed<ChartOptions<'line'>>(() => ({
  responsive: true, maintainAspectRatio: false, animation: false,
  interaction: { mode: 'index', intersect: false },
  plugins: { legend: { display: false }, tooltip: { callbacks: {
    title: items => {
      const point = props.points[items[0]?.dataIndex ?? 0]
      return point ? new Date(point.at).toLocaleString(locale.value, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }) : ''
    },
    label: item => `${item.dataset.label}: ${Number(item.raw).toLocaleString(locale.value)}`,
  } } },
  scales: {
    x: { grid: { display: false }, ticks: { color: '#94a3b8', maxTicksLimit: 7, maxRotation: 0 }, border: { display: false } },
    y: { beginAtZero: true, grid: { color: 'rgba(148,163,184,0.12)' }, ticks: { color: '#94a3b8', maxTicksLimit: 5, precision: 0, callback: value => compact(Number(value)) }, border: { display: false } },
  },
}))
</script>

<template>
  <div class="h-56 sm:h-64">
    <Line :data="chartData" :options="options" :aria-label="t('groupRates.trendTitle')" role="img" />
  </div>
</template>
