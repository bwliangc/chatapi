<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="card p-4">
        <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
          <div class="min-w-0">
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">
              {{ t('admin.costCalculator.statementTitle') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
              {{ t('admin.costCalculator.statementDescription') }}
            </p>
          </div>
          <div class="flex flex-wrap items-center gap-3">
            <DateRangePicker
              v-model:start-date="startDate"
              v-model:end-date="endDate"
              @change="refreshData"
            />
            <button type="button" class="btn btn-secondary" :disabled="loading" @click="refreshData">
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
              {{ t('common.refresh') }}
            </button>
            <button type="button" class="btn btn-secondary" @click="openSettingsDialog">
              <Icon name="cog" size="md" />
              {{ t('admin.costCalculator.settings') }}
            </button>
          </div>
        </div>
      </section>

      <div class="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4">
        <MetricCard
          :label="t('admin.costCalculator.userBilling')"
          :value="formatActualMoney(userBillingRevenue)"
          :hint="t('admin.costCalculator.userBillingHint', { rate: formatRmbPerUsd(config.balance_exchange_rate) })"
          tone="primary"
        />
        <MetricCard
          :label="t('admin.costCalculator.upstreamCost')"
          :value="formatActualMoney(upstreamCost)"
          :hint="t('admin.costCalculator.upstreamCostHint', { rate: formatCompositeUsageRate(config.upstream_cost_rate) })"
          tone="amber"
        />
        <MetricCard
          :label="t('admin.costCalculator.usageGrossProfit')"
          :value="formatActualMoney(usageGrossProfit)"
          :hint="`${t('admin.costCalculator.margin')} ${formatPercent(usageGrossMargin)}`"
          :tone="usageGrossProfit >= 0 ? 'emerald' : 'red'"
        />
        <MetricCard
          :label="t('admin.costCalculator.netAfterFixedCost')"
          :value="formatActualMoney(netProfitAfterFixedCost)"
          :hint="t('admin.costCalculator.netAfterFixedCostHint', { fixed: formatActualMoney(proratedFixedCost), rewards: formatActualMoney(leaderboardRewardCost) })"
          :tone="netProfitAfterFixedCost >= 0 ? 'emerald' : 'red'"
        />
      </div>

      <div class="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4">
        <MetricCard
          :label="t('admin.costCalculator.userBalanceLiability')"
          :value="formatPlatformBalance(balanceLiabilityBalance)"
          :hint="t('admin.costCalculator.userBalanceLiabilityHint', { count: balanceLiabilityUserCount, estimated: formatEstimatedBalanceLiability(), source: balanceLiabilitySourceLabel })"
        />
        <MetricCard
          :label="t('admin.costCalculator.balanceRechargeRevenue')"
          :value="formatActualMoney(balanceRechargeRevenue)"
          :hint="t('admin.costCalculator.balanceRechargeRevenueHint', { count: balanceRechargeRecordCount, matched: formatPlatformBalance(matchedBalanceAmount), unmatched: formatPlatformBalance(unmatchedBalanceAmount) })"
        />
        <MetricCard
          :label="t('admin.costCalculator.redeemRechargeRevenue')"
          :value="formatActualMoney(redeemRechargeRevenue)"
          :hint="t('admin.costCalculator.redeemRechargeRevenueHint', { count: redeemRechargeCount })"
        />
        <MetricCard
          :label="t('admin.costCalculator.adminBalanceAdjustment')"
          :value="formatActualMoney(adminBalanceAdjustment)"
          :hint="t('admin.costCalculator.adminBalanceAdjustmentHint', { count: adminBalanceAdjustmentCount, added: formatPlatformBalance(adminBalanceAdded), deducted: formatPlatformBalance(adminBalanceDeducted) })"
        />
        <MetricCard
          :label="t('admin.costCalculator.leaderboardRewardCost')"
          :value="formatActualMoney(leaderboardRewardCost)"
          :hint="t('admin.costCalculator.leaderboardRewardCostHint', { count: leaderboardRewardCount, amount: formatPlatformBalance(leaderboardRewardAmount) })"
          tone="amber"
        />
        <MetricCard
          v-if="unmatchedBalanceRecordCount > 0"
          :label="t('admin.costCalculator.unmatchedRechargeAmount')"
          :value="formatPlatformBalance(unmatchedBalanceAmount)"
          :hint="t('admin.costCalculator.unmatchedRechargeAmountHint', { count: unmatchedBalanceRecordCount })"
          tone="red"
        />
        <MetricCard
          :label="t('admin.costCalculator.accountConfiguredCost')"
          :value="formatActualMoney(monthlyFixedCost)"
          :hint="t('admin.costCalculator.accountConfiguredCostHint', { count: configuredCostAccounts })"
        />
      </div>

      <div class="grid grid-cols-1 gap-6 2xl:grid-cols-[minmax(0,1.2fr)_minmax(360px,0.8fr)]">
        <section class="card overflow-hidden">
          <div class="flex flex-col gap-3 border-b border-gray-200 px-5 py-4 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h2 class="text-base font-semibold text-gray-900 dark:text-white">
                {{ t('admin.costCalculator.accountCostTitle') }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
                {{ t('admin.costCalculator.accountCostDescription') }}
              </p>
            </div>
            <span class="text-sm text-gray-500 dark:text-dark-400">
              {{ t('admin.costCalculator.proratedDays', { days: selectedDays }) }}
            </span>
          </div>
          <div class="overflow-x-auto">
            <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
              <thead class="bg-gray-50 dark:bg-dark-800/70">
                <tr>
                  <th class="px-5 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">
                    {{ t('admin.costCalculator.account') }}
                  </th>
                  <th class="px-5 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">
                    {{ t('admin.costCalculator.platform') }}
                  </th>
                  <th class="px-5 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">
                    {{ t('admin.costCalculator.income') }}
                  </th>
                  <th class="px-5 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">
                    {{ t('admin.costCalculator.periodUsageCost') }}
                  </th>
                  <th class="px-5 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">
                    {{ t('admin.costCalculator.fixedCost') }}
                  </th>
                  <th class="px-5 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">
                    {{ t('admin.costCalculator.cost') }}
                  </th>
                  <th class="px-5 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">
                    {{ t('admin.costCalculator.profit') }}
                  </th>
                  <th class="px-5 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">
                    {{ t('admin.costCalculator.costNote') }}
                  </th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 bg-white/60 dark:divide-dark-700 dark:bg-dark-800/30">
                <tr v-if="settingsLoading">
                  <td colspan="8" class="px-5 py-8 text-center text-sm text-gray-500 dark:text-dark-400">
                    {{ t('common.loading') }}
                  </td>
                </tr>
                <tr v-else-if="accountRows.length === 0">
                  <td colspan="8" class="px-5 py-8 text-center text-sm text-gray-500 dark:text-dark-400">
                    {{ t('admin.costCalculator.noAccounts') }}
                  </td>
                </tr>
                <tr v-for="row in accountRows" v-else :key="row.id">
                  <td class="px-5 py-3">
                    <div class="min-w-0">
                      <div class="truncate text-sm font-medium text-gray-900 dark:text-white">{{ row.name }}</div>
                      <div class="text-xs text-gray-500 dark:text-dark-400">#{{ row.id }} · {{ row.type }}</div>
                    </div>
                  </td>
                  <td class="whitespace-nowrap px-5 py-3 text-sm text-gray-600 dark:text-dark-300">
                    {{ row.platform }}
                  </td>
                  <td class="whitespace-nowrap px-5 py-3 text-right font-mono text-sm text-gray-900 dark:text-white">
                    {{ formatActualMoney(row.period_income) }}
                  </td>
                  <td class="whitespace-nowrap px-5 py-3 text-right font-mono text-sm text-gray-900 dark:text-white">
                    {{ formatActualMoney(row.period_usage_cost) }}
                  </td>
                  <td class="whitespace-nowrap px-5 py-3 text-right font-mono text-sm text-gray-900 dark:text-white">
                    <div>{{ formatActualMoney(row.period_fixed_cost) }}</div>
                    <div class="text-xs text-gray-500 dark:text-dark-400">{{ formatActualMoney(row.monthly_fixed_cost) }}/月</div>
                  </td>
                  <td class="whitespace-nowrap px-5 py-3 text-right font-mono text-sm text-gray-900 dark:text-white">
                    {{ formatActualMoney(row.total_cost) }}
                  </td>
                  <td class="whitespace-nowrap px-5 py-3 text-right font-mono text-sm" :class="profitToneClass(row.profit)">
                    {{ formatActualMoney(row.profit) }}
                  </td>
                  <td class="px-5 py-3 text-sm text-gray-600 dark:text-dark-300">
                    <div>{{ row.note || '-' }}</div>
                    <div class="text-xs text-gray-500 dark:text-dark-400">{{ formatCompositeUsageRate(row.usage_cost_rate) }}</div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>

        <section class="card overflow-hidden">
          <div class="border-b border-gray-200 px-5 py-4 dark:border-dark-700">
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">
              {{ t('admin.costCalculator.accountingNotesTitle') }}
            </h2>
          </div>
          <div class="space-y-4 p-5">
            <InfoRow
              :label="t('admin.costCalculator.revenueFormula')"
              :value="t('admin.costCalculator.revenueFormulaValue')"
            />
            <InfoRow
              :label="t('admin.costCalculator.costFormula')"
              :value="t('admin.costCalculator.costFormulaValue')"
            />
            <InfoRow
              :label="t('admin.costCalculator.fixedCostSource')"
              :value="t('admin.costCalculator.fixedCostSourceValue')"
            />
            <InfoRow
              :label="t('admin.costCalculator.balanceLiabilityFormula')"
              :value="t('admin.costCalculator.balanceLiabilityFormulaValue')"
            />
          </div>
        </section>
      </div>

      <section class="card overflow-hidden">
        <div class="flex flex-col gap-3 border-b border-gray-200 px-5 py-4 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">
              {{ t('admin.costCalculator.balanceLiabilityTitle') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
              {{ t('admin.costCalculator.balanceLiabilityDescription') }}
            </p>
          </div>
          <span class="text-sm text-gray-500 dark:text-dark-400">
            {{ t('admin.costCalculator.pageLoadedRows', { count: filteredUsers.length, total: users.length }) }}
          </span>
        </div>
        <div class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-800/70">
              <tr>
                <th class="px-5 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">
                  {{ t('admin.costCalculator.user') }}
                </th>
                <th class="px-5 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">
                  {{ t('common.balance') }}
                </th>
                <th class="px-5 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">
                  {{ t('admin.costCalculator.subscriptions') }}
                </th>
                <th class="px-5 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">
                  {{ t('admin.users.status') }}
                </th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 bg-white/60 dark:divide-dark-700 dark:bg-dark-800/30">
              <tr v-if="usersLoading">
                <td colspan="4" class="px-5 py-8 text-center text-sm text-gray-500 dark:text-dark-400">
                  {{ t('common.loading') }}
                </td>
              </tr>
              <tr v-for="user in topBalanceUsers" v-else :key="user.id">
                <td class="px-5 py-3">
                  <div class="text-sm font-medium text-gray-900 dark:text-white">{{ user.email }}</div>
                  <div class="text-xs text-gray-500 dark:text-dark-400">#{{ user.id }}</div>
                </td>
                <td class="px-5 py-3 text-right font-mono text-sm text-gray-900 dark:text-white">
                  {{ formatPlatformBalance(user.balance) }}
                </td>
                <td class="px-5 py-3 text-right font-mono text-sm text-gray-600 dark:text-dark-300">
                  {{ user.subscriptions?.length || 0 }}
                </td>
                <td class="px-5 py-3 text-right text-sm">
                  <span :class="['rounded px-2 py-1 text-xs font-medium', user.status === 'active' ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-300' : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-dark-300']">
                    {{ user.status }}
                  </span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <BaseDialog
        :show="showSettingsDialog"
        :title="t('admin.costCalculator.settingsTitle')"
        width="wide"
        @close="closeSettingsDialog"
      >
        <div class="space-y-6">
          <section class="grid grid-cols-1 gap-4 md:grid-cols-2">
            <div>
              <label class="input-label">{{ t('admin.costCalculator.balanceExchangeRate') }}</label>
              <input
                v-model.number="settingsForm.balance_exchange_rate"
                type="number"
                min="0.0001"
                step="0.0001"
                class="input"
              />
              <p class="input-hint">{{ t('admin.costCalculator.balanceExchangeRateHint') }}</p>
            </div>
            <div>
              <label class="input-label">{{ t('admin.costCalculator.upstreamCostRate') }}</label>
              <input
                v-model.number="settingsForm.upstream_cost_rate"
                type="number"
                min="0"
                step="0.0001"
                class="input"
              />
              <p class="input-hint">{{ t('admin.costCalculator.upstreamCostRateHint') }}</p>
            </div>
          </section>

          <section class="space-y-3">
            <div class="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
              <div>
                <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
                  {{ t('admin.costCalculator.rechargePackageSettings') }}
                </h3>
                <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
                  {{ t('admin.costCalculator.rechargePackageSettingsHint') }}
                </p>
              </div>
              <button type="button" class="btn btn-secondary" @click="addBalanceRechargePackage">
                <Icon name="plus" size="md" />
                {{ t('admin.costCalculator.addRechargePackage') }}
              </button>
            </div>

            <div class="overflow-x-auto rounded-lg border border-gray-200 dark:border-dark-700">
              <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
                <thead class="bg-gray-50 dark:bg-dark-800/70">
                  <tr>
                    <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">
                      {{ t('admin.costCalculator.rechargeBalanceAmount') }}
                    </th>
                    <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">
                      {{ t('admin.costCalculator.rechargeActualAmount') }}
                    </th>
                    <th class="w-16 px-4 py-3"></th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-700 dark:bg-dark-800">
                  <tr v-if="settingsForm.balance_recharge_packages.length === 0">
                    <td colspan="3" class="px-4 py-6 text-center text-sm text-gray-500 dark:text-dark-400">
                      {{ t('admin.costCalculator.noRechargePackages') }}
                    </td>
                  </tr>
                  <tr v-for="(item, index) in settingsForm.balance_recharge_packages" v-else :key="index">
                    <td class="px-4 py-3">
                      <input
                        v-model.number="item.balance_amount"
                        type="number"
                        min="0.000001"
                        step="0.000001"
                        class="input text-right font-mono"
                      />
                    </td>
                    <td class="px-4 py-3">
                      <input
                        v-model.number="item.actual_amount"
                        type="number"
                        min="0"
                        step="0.01"
                        class="input text-right font-mono"
                      />
                    </td>
                    <td class="px-4 py-3 text-right">
                      <button
                        type="button"
                        class="rounded p-2 text-gray-400 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-500/10 dark:hover:text-red-400"
                        :title="t('common.delete')"
                        @click="removeBalanceRechargePackage(index)"
                      >
                        <Icon name="trash" size="sm" />
                      </button>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>

          <section class="space-y-3">
            <div class="flex flex-col gap-3 sm:flex-row sm:items-end">
              <div class="flex-1">
                <label class="input-label">{{ t('admin.costCalculator.addAccountCost') }}</label>
                <select v-model.number="selectedAccountId" class="input">
                  <option :value="0">{{ t('admin.costCalculator.selectAccount') }}</option>
                  <option
                    v-for="account in availableAccountsForCost"
                    :key="account.id"
                    :value="account.id"
                  >
                    #{{ account.id }} · {{ account.name }} · {{ account.platform }}
                  </option>
                </select>
              </div>
              <button
                type="button"
                class="btn btn-secondary"
                :disabled="selectedAccountId <= 0"
                @click="addAccountCost"
              >
                <Icon name="plus" size="md" />
                {{ t('common.add') }}
              </button>
            </div>

            <div class="overflow-x-auto rounded-lg border border-gray-200 dark:border-dark-700">
              <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
                <thead class="bg-gray-50 dark:bg-dark-800/70">
                  <tr>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">
                      {{ t('admin.costCalculator.account') }}
                    </th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">
                      {{ t('admin.costCalculator.platform') }}
                    </th>
                    <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">
                      {{ t('admin.costCalculator.monthlyFixedCost') }}
                    </th>
                    <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">
                      {{ t('admin.costCalculator.usageCostRate') }}
                    </th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">
                      {{ t('admin.costCalculator.costNote') }}
                    </th>
                    <th class="w-16 px-4 py-3"></th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-700 dark:bg-dark-800">
                  <tr v-if="settingsForm.account_costs.length === 0">
                    <td colspan="6" class="px-4 py-6 text-center text-sm text-gray-500 dark:text-dark-400">
                      {{ t('admin.costCalculator.noAccountCosts') }}
                    </td>
                  </tr>
                  <tr v-for="(item, index) in settingsForm.account_costs" v-else :key="item.account_id">
                    <td class="px-4 py-3 text-sm text-gray-900 dark:text-white">
                      <div class="font-medium">{{ item.account_name || accountName(item.account_id) }}</div>
                      <div class="text-xs text-gray-500 dark:text-dark-400">#{{ item.account_id }}</div>
                    </td>
                    <td class="px-4 py-3 text-sm text-gray-600 dark:text-dark-300">
                      {{ item.platform || accountPlatform(item.account_id) }}
                    </td>
                    <td class="px-4 py-3">
                      <input
                        v-model.number="item.monthly_cost"
                        type="number"
                        min="0"
                        step="0.01"
                        class="input text-right font-mono"
                      />
                    </td>
                    <td class="px-4 py-3">
                      <input
                        v-model.number="item.usage_cost_rate"
                        type="number"
                        min="0"
                        step="0.0001"
                        class="input text-right font-mono"
                      />
                    </td>
                    <td class="px-4 py-3">
                      <input
                        v-model.trim="item.monthly_cost_label"
                        type="text"
                        class="input"
                        :placeholder="t('admin.costCalculator.costNotePlaceholder')"
                      />
                    </td>
                    <td class="px-4 py-3 text-right">
                      <button
                        type="button"
                        class="rounded p-2 text-gray-400 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-500/10 dark:hover:text-red-400"
                        :title="t('common.delete')"
                        @click="removeAccountCost(index)"
                      >
                        <Icon name="trash" size="sm" />
                      </button>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>
        </div>
        <template #footer>
          <button type="button" class="btn btn-secondary" :disabled="settingsSaving" @click="closeSettingsDialog">
            {{ t('common.cancel') }}
          </button>
          <button type="button" class="btn btn-primary" :disabled="settingsSaving" @click="saveSettings">
            {{ settingsSaving ? t('common.saving') : t('common.save') }}
          </button>
        </template>
      </BaseDialog>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import usersAPI from '@/api/admin/users'
import accountsAPI from '@/api/admin/accounts'
import {
  costCalculatorAPI,
  type CostCalculatorAccountCost,
  type CostCalculatorAccountUsage,
  type CostCalculatorBalanceLiabilitySummary,
  type CostCalculatorBalanceRechargePackage,
  type CostCalculatorBalanceRechargeSummary,
  type CostCalculatorConfig,
  type CostCalculatorUsageSummary
} from '@/api/admin/costCalculator'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { Account, AdminUser } from '@/types'

interface AccountCostRow {
  id: number
  name: string
  platform: string
  type: string
  monthly_fixed_cost: number
  usage_cost_rate: number
  period_income: number
  period_usage_cost: number
  period_fixed_cost: number
  total_cost: number
  profit: number
  note: string
}

interface MetricToneClasses {
  value: string
  accent: string
}

const MetricCard = defineComponent({
  name: 'MetricCard',
  props: {
    label: { type: String, required: true },
    value: { type: String, required: true },
    hint: { type: String, default: '' },
    tone: { type: String, default: 'default' }
  },
  setup(props) {
    const toneClasses = computed<MetricToneClasses>(() => {
      switch (props.tone) {
        case 'primary':
          return { value: 'text-primary-600 dark:text-primary-400', accent: 'bg-primary-500' }
        case 'amber':
          return { value: 'text-amber-600 dark:text-amber-400', accent: 'bg-amber-500' }
        case 'emerald':
          return { value: 'text-emerald-600 dark:text-emerald-400', accent: 'bg-emerald-500' }
        case 'red':
          return { value: 'text-red-600 dark:text-red-400', accent: 'bg-red-500' }
        default:
          return { value: 'text-gray-900 dark:text-white', accent: 'bg-gray-300 dark:bg-dark-500' }
      }
    })

    return () => h('div', { class: 'card p-5' }, [
      h('div', { class: 'flex items-center gap-2' }, [
        h('span', { class: ['h-2 w-2 rounded-full', toneClasses.value.accent] }),
        h('p', { class: 'text-xs font-medium text-gray-500 dark:text-dark-400' }, props.label)
      ]),
      h('p', { class: ['mt-2 font-mono text-2xl font-semibold', toneClasses.value.value] }, props.value),
      props.hint ? h('p', { class: 'mt-1 text-xs text-gray-500 dark:text-dark-400' }, props.hint) : null
    ])
  }
})

const InfoRow = defineComponent({
  name: 'InfoRow',
  props: {
    label: { type: String, required: true },
    value: { type: String, required: true }
  },
  setup(props) {
    return () => h('div', { class: 'rounded-lg border border-gray-200 bg-white/70 p-3 dark:border-dark-600 dark:bg-dark-800/60' }, [
      h('p', { class: 'text-xs font-medium text-gray-500 dark:text-dark-400' }, props.label),
      h('p', { class: 'mt-1 text-sm text-gray-900 dark:text-white' }, props.value)
    ])
  }
})

const { t } = useI18n()
const appStore = useAppStore()

const startDate = ref(defaultStartDate())
const endDate = ref(defaultEndDate())
const loading = ref(false)
const usersLoading = ref(false)
const settingsLoading = ref(false)
const settingsSaving = ref(false)
const showSettingsDialog = ref(false)
const selectedAccountId = ref(0)
const usageSummary = ref<CostCalculatorUsageSummary | null>(null)
const users = ref<AdminUser[]>([])
const accounts = ref<Account[]>([])
const balanceRechargeSummary = ref<CostCalculatorBalanceRechargeSummary | null>(null)
const balanceLiabilitySummary = ref<CostCalculatorBalanceLiabilitySummary | null>(null)
const config = reactive<CostCalculatorConfig>(defaultCostCalculatorConfig())
const settingsForm = reactive<CostCalculatorConfig>(defaultCostCalculatorConfig())

const defaultCompositeUsageRate = computed(() =>
  nonNegativeOrDefault(config.upstream_cost_rate, positiveOrDefault(config.balance_exchange_rate, 1))
)

const selectedDays = computed(() => {
  const start = new Date(`${startDate.value}T00:00:00`)
  const end = new Date(`${endDate.value}T00:00:00`)
  if (!Number.isFinite(start.getTime()) || !Number.isFinite(end.getTime())) return 0
  const ms = end.getTime() - start.getTime()
  return Math.max(1, Math.floor(ms / 86_400_000) + 1)
})

const rawUserBilling = computed(() => toFinite(usageSummary.value?.total_actual_cost))
const userBillingRevenue = computed(() => balanceToRevenue(rawUserBilling.value))
const upstreamCost = computed(() => toFinite(usageSummary.value?.total_usage_cost))
const usageGrossProfit = computed(() => userBillingRevenue.value - upstreamCost.value)
const usageGrossMargin = computed(() => {
  return userBillingRevenue.value > 0 ? usageGrossProfit.value / userBillingRevenue.value : 0
})
const monthlyFixedCost = computed(() =>
  accountRows.value.reduce((sum, row) => sum + row.monthly_fixed_cost, 0)
)
const proratedFixedCost = computed(() => monthlyFixedCost.value / 30 * selectedDays.value)
const leaderboardRewardAmount = computed(() => toFinite(balanceRechargeSummary.value?.leaderboard_reward_amount))
const leaderboardRewardCost = computed(() => balanceToRevenue(leaderboardRewardAmount.value))
const leaderboardRewardCount = computed(() => toFinite(balanceRechargeSummary.value?.leaderboard_reward_count))
const netProfitAfterFixedCost = computed(() => usageGrossProfit.value - proratedFixedCost.value - leaderboardRewardCost.value)
const configuredCostAccounts = computed(() => accountRows.value.length)
const balanceRechargeRevenue = computed(() => toFinite(balanceRechargeSummary.value?.actual_revenue))
const redeemRechargeRevenue = computed(() => toFinite(balanceRechargeSummary.value?.redeem_actual_revenue))
const adminBalanceAdjustment = computed(() => toFinite(balanceRechargeSummary.value?.admin_actual_revenue))
const adminBalanceAdded = computed(() => toFinite(balanceRechargeSummary.value?.admin_added_amount))
const adminBalanceDeducted = computed(() => toFinite(balanceRechargeSummary.value?.admin_deducted_amount))
const matchedBalanceAmount = computed(() => toFinite(balanceRechargeSummary.value?.matched_balance_amount))
const unmatchedBalanceAmount = computed(() => toFinite(balanceRechargeSummary.value?.unmatched_balance_amount))
const unmatchedBalanceRecordCount = computed(() => toFinite(balanceRechargeSummary.value?.unmatched_record_count))
const balanceRechargeRecordCount = computed(() => toFinite(balanceRechargeSummary.value?.record_count))
const redeemRechargeCount = computed(() => toFinite(balanceRechargeSummary.value?.redeem_count))
const adminBalanceAdjustmentCount = computed(() => toFinite(balanceRechargeSummary.value?.admin_count))
const balanceLiabilityBalance = computed(() => toFinite(balanceLiabilitySummary.value?.total_balance))
const balanceLiabilityEstimatedActual = computed(() => toFinite(balanceLiabilitySummary.value?.estimated_actual_liability))
const balanceLiabilityEstimatedUnitCost = computed(() => toFinite(balanceLiabilitySummary.value?.estimated_unit_cost))
// 套餐价格表的加权售价单价（Σ实付 / Σ面额），用于把「平台余额额度」折算成人民币收入。
// 收入口径必须用售价，而非负债估值单价（EstimatedUnitCost）——后者是全平台历史均价，
// 拿来当各分组收入单价会在售价/成本结构不同的分组间产生假性盈亏。
const salePriceUnitCost = computed(() => {
  let balanceTotal = 0
  let actualTotal = 0
  for (const item of config.balance_recharge_packages) {
    const balance = toFinite(item.balance_amount)
    const actual = toFinite(item.actual_amount)
    if (balance <= 0 || actual < 0) continue
    balanceTotal += balance
    actualTotal += actual
  }
  if (balanceTotal > 0) return actualTotal / balanceTotal
  // 套餐表不可用时回退到负债估值单价，再回退到汇率。
  if (hasBalanceLiabilityValuation.value && balanceLiabilityEstimatedUnitCost.value > 0) {
    return balanceLiabilityEstimatedUnitCost.value
  }
  return positiveOrDefault(config.balance_exchange_rate, 1)
})
const balanceLiabilityUserCount = computed(() => toFinite(balanceLiabilitySummary.value?.positive_user_count))
const hasBalanceLiabilityValuation = computed(() => {
  const source = balanceLiabilitySummary.value?.valuation_source
  return source === 'matched_recharge_history' || source === 'package_table'
})
const balanceLiabilitySourceLabel = computed(() => {
  switch (balanceLiabilitySummary.value?.valuation_source) {
    case 'matched_recharge_history':
      return t('admin.costCalculator.balanceLiabilitySourceMatched')
    case 'package_table':
      return t('admin.costCalculator.balanceLiabilitySourcePackage')
    default:
      return t('admin.costCalculator.balanceLiabilitySourceUnavailable')
  }
})

const topBalanceUsers = computed(() =>
  [...filteredUsers.value]
    .sort((a, b) => toFinite(b.balance) - toFinite(a.balance))
    .slice(0, 10)
)

const filteredUsers = computed(() => users.value.filter(user => user.role !== 'admin'))

const accountsById = computed(() => {
  const map = new Map<number, Account>()
  for (const account of accounts.value) {
    map.set(account.id, account)
  }
  return map
})

const accountRows = computed<AccountCostRow[]>(() => {
  const accountIds = new Set<number>()
  for (const item of config.account_costs) {
    if (item.account_id > 0) accountIds.add(item.account_id)
  }
  for (const usage of usageSummary.value?.accounts || []) {
    if (usage.account_id > 0) accountIds.add(usage.account_id)
  }
  return [...accountIds].map(accountId => {
    const configured = config.account_costs.find(item => item.account_id === accountId)
    const account = accountsById.value.get(accountId)
    const usage = accountUsageById.value.get(accountId)
    const monthlyFixedCost = nonNegativeOrDefault(configured?.monthly_cost, 0)
    const periodFixedCost = monthlyFixedCost / 30 * selectedDays.value
    const periodUsageCost = toFinite(usage?.usage_cost)
    const periodIncome = balanceToRevenue(usage?.actual_cost)
    const totalCost = periodUsageCost + periodFixedCost
    return {
      id: accountId,
      name: configured?.account_name || usage?.account_name || account?.name || `#${accountId}`,
      platform: configured?.platform || usage?.platform || account?.platform || '-',
      type: account?.type || '-',
      monthly_fixed_cost: monthlyFixedCost,
      usage_cost_rate: accountUsageCostRate(configured || {}),
      period_income: periodIncome,
      period_usage_cost: periodUsageCost,
      period_fixed_cost: periodFixedCost,
      total_cost: totalCost,
      profit: periodIncome - totalCost,
      note: configured?.monthly_cost_label || ''
    }
  }).sort((a, b) => b.total_cost - a.total_cost || b.period_income - a.period_income || a.id - b.id)
})

const accountUsageById = computed(() => {
  const map = new Map<number, CostCalculatorAccountUsage>()
  for (const row of usageSummary.value?.accounts || []) {
    map.set(row.account_id, row)
  }
  return map
})

const availableAccountsForCost = computed(() => {
  const configured = new Set(settingsForm.account_costs.map(item => Number(item.account_id)))
  return accounts.value
    .filter(account => !configured.has(account.id))
    .sort((a, b) => a.id - b.id)
})

onMounted(async () => {
  await refreshData()
})

async function refreshData() {
  loading.value = true
  try {
    await Promise.all([
      loadSettings(),
      loadUsageFinance(),
      loadUsers(),
      loadAccounts(),
      loadBalanceRechargeSummary(),
      loadBalanceLiabilitySummary()
    ])
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  } finally {
    loading.value = false
  }
}

async function loadUsageFinance() {
  const params = buildAccountingStatsParams()
  const summary = await costCalculatorAPI.getUsageSummary(params)
  usageSummary.value = summary
}

async function loadUsers() {
  usersLoading.value = true
  try {
    const resp = await usersAPI.list(1, 200, {
      include_subscriptions: true,
      role: 'user',
      sort_by: 'balance',
      sort_order: 'desc'
    })
    users.value = resp.items || []
  } finally {
    usersLoading.value = false
  }
}

async function loadAccounts() {
  const resp = await accountsAPI.list(1, 200, undefined)
  accounts.value = resp.items || []
}

async function loadSettings() {
  settingsLoading.value = true
  try {
    const nextConfig = await costCalculatorAPI.getConfig()
    applyConfig(config, nextConfig)
    applyConfig(settingsForm, config)
  } finally {
    settingsLoading.value = false
  }
}

async function loadBalanceRechargeSummary() {
  try {
    balanceRechargeSummary.value = await costCalculatorAPI.getBalanceRechargeSummary(buildAccountingStatsParams())
  } catch {
    balanceRechargeSummary.value = null
  }
}

async function loadBalanceLiabilitySummary() {
  try {
    balanceLiabilitySummary.value = await costCalculatorAPI.getBalanceLiabilitySummary(buildAccountingStatsParams())
  } catch {
    balanceLiabilitySummary.value = null
  }
}

function buildAccountingStatsParams() {
  return {
    start_date: startDate.value,
    end_date: endDate.value,
    nocache: Date.now(),
    exclude_admins: true
  }
}

function profitToneClass(value: number): string {
  return value >= 0 ? 'text-emerald-600 dark:text-emerald-400' : 'text-red-600 dark:text-red-400'
}

// 把「平台余额额度」按售价折算成人民币收入/机会成本。
// 用于：账号收入、顶部用户计费、排行榜奖励成本（送出额度的机会成本）。
// 收入侧统一用售价单价（salePriceUnitCost），区别于负债估值口径。
function balanceToRevenue(value: unknown): number {
  return toFinite(value) * salePriceUnitCost.value
}

function openSettingsDialog() {
  applyConfig(settingsForm, config)
  selectedAccountId.value = 0
  showSettingsDialog.value = true
  if (accounts.value.length === 0) {
    void loadAccounts()
  }
}

function closeSettingsDialog() {
  if (settingsSaving.value) return
  showSettingsDialog.value = false
  selectedAccountId.value = 0
}

function addAccountCost() {
  const id = Number(selectedAccountId.value)
  if (id <= 0 || settingsForm.account_costs.some(item => item.account_id === id)) return
  const account = accountsById.value.get(id)
  settingsForm.account_costs.push({
    account_id: id,
    account_name: account?.name || '',
    platform: account?.platform || '',
    monthly_cost: 0,
    usage_cost_rate: nonNegativeOrDefault(settingsForm.upstream_cost_rate, positiveOrDefault(settingsForm.balance_exchange_rate, 1)),
    monthly_cost_label: ''
  })
  selectedAccountId.value = 0
}

function removeAccountCost(index: number) {
  settingsForm.account_costs.splice(index, 1)
}

function addBalanceRechargePackage() {
  settingsForm.balance_recharge_packages.push({
    balance_amount: 0,
    actual_amount: 0
  })
}

function removeBalanceRechargePackage(index: number) {
  settingsForm.balance_recharge_packages.splice(index, 1)
}

async function saveSettings() {
  if (toFinite(settingsForm.balance_exchange_rate) <= 0) {
    appStore.showError(t('admin.costCalculator.balanceExchangeRateInvalid'))
    return
  }
  if (toFinite(settingsForm.upstream_cost_rate) < 0) {
    appStore.showError(t('admin.costCalculator.upstreamCostRateInvalid'))
    return
  }
  if (settingsForm.account_costs.some(item => toFinite(item.monthly_cost) < 0)) {
    appStore.showError(t('admin.costCalculator.monthlyCostInvalid'))
    return
  }
  if (settingsForm.account_costs.some(item => toFinite(item.usage_cost_rate) < 0)) {
    appStore.showError(t('admin.costCalculator.accountUsageCostRateInvalid'))
    return
  }
  if (settingsForm.balance_recharge_packages.some(item => toFinite(item.balance_amount) <= 0)) {
    appStore.showError(t('admin.costCalculator.rechargePackageBalanceInvalid'))
    return
  }
  if (settingsForm.balance_recharge_packages.some(item => toFinite(item.actual_amount) < 0)) {
    appStore.showError(t('admin.costCalculator.rechargePackageActualInvalid'))
    return
  }
  if (hasDuplicateRechargePackage(settingsForm.balance_recharge_packages)) {
    appStore.showError(t('admin.costCalculator.rechargePackageDuplicate'))
    return
  }

  const nextConfig = normalizeConfig(settingsForm)
  nextConfig.account_costs = nextConfig.account_costs.map(item => ({
    ...item,
    account_name: accountName(item.account_id),
    platform: accountPlatform(item.account_id)
  }))

  settingsSaving.value = true
  try {
    const saved = await costCalculatorAPI.updateConfig(nextConfig)
    applyConfig(config, saved)
    applyConfig(settingsForm, saved)
    showSettingsDialog.value = false
    appStore.showSuccess(t('admin.costCalculator.settingsSaved'))
    try {
      await Promise.all([loadUsageFinance(), loadBalanceRechargeSummary(), loadBalanceLiabilitySummary()])
    } catch (err: unknown) {
      appStore.showError(extractApiErrorMessage(err, t('admin.costCalculator.usageSummaryLoadFailed')))
    }
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.costCalculator.settingsSaveFailed')))
  } finally {
    settingsSaving.value = false
  }
}

function accountName(accountId: number): string {
  const account = accountsById.value.get(Number(accountId))
  return account?.name || settingsForm.account_costs.find(item => item.account_id === accountId)?.account_name || ''
}

function accountPlatform(accountId: number): string {
  const account = accountsById.value.get(Number(accountId))
  return account?.platform || settingsForm.account_costs.find(item => item.account_id === accountId)?.platform || ''
}

function accountUsageCostRate(item: Partial<CostCalculatorAccountCost>): number {
  return nonNegativeOrDefault(item.usage_cost_rate, defaultCompositeUsageRate.value)
}

function applyConfig(target: CostCalculatorConfig, source: CostCalculatorConfig) {
  const normalized = normalizeConfig(source)
  target.balance_exchange_rate = normalized.balance_exchange_rate
  target.upstream_cost_rate = normalized.upstream_cost_rate
  target.account_costs.splice(0, target.account_costs.length, ...normalized.account_costs)
  target.balance_recharge_packages.splice(0, target.balance_recharge_packages.length, ...normalized.balance_recharge_packages)
}

function normalizeConfig(source: Partial<CostCalculatorConfig> | null | undefined): CostCalculatorConfig {
  const seen = new Set<number>()
  const accountCosts = Array.isArray(source?.account_costs) ? source.account_costs : []
  const rechargePackages = Array.isArray(source?.balance_recharge_packages) ? source.balance_recharge_packages : []
  const defaultUsageCostRate = nonNegativeOrDefault(
    source?.upstream_cost_rate,
    positiveOrDefault(source?.balance_exchange_rate, 1)
  )
  return {
    balance_exchange_rate: positiveOrDefault(source?.balance_exchange_rate, 1),
    upstream_cost_rate: defaultUsageCostRate,
    account_costs: accountCosts
      .map(item => cloneAccountCost(item, defaultUsageCostRate))
      .filter(item => {
        if (item.account_id <= 0 || seen.has(item.account_id)) return false
        seen.add(item.account_id)
        return true
      }),
    balance_recharge_packages: normalizeRechargePackages(rechargePackages)
  }
}

function cloneAccountCost(item: Partial<CostCalculatorAccountCost>, defaultUsageCostRate = 1): CostCalculatorAccountCost {
  return {
    account_id: Math.trunc(nonNegativeOrDefault(item.account_id, 0)),
    account_name: String(item.account_name || '').trim(),
    platform: String(item.platform || '').trim(),
    monthly_cost: nonNegativeOrDefault(item.monthly_cost, 0),
    usage_cost_rate: nonNegativeOrDefault(item.usage_cost_rate, defaultUsageCostRate),
    monthly_cost_label: String(item.monthly_cost_label || '').trim()
  }
}

function normalizeRechargePackages(items: Partial<CostCalculatorBalanceRechargePackage>[]): CostCalculatorBalanceRechargePackage[] {
  const seen = new Set<number>()
  return items
    .map(item => ({
      balance_amount: positiveOrDefault(item.balance_amount, 0),
      actual_amount: nonNegativeOrDefault(item.actual_amount, 0)
    }))
    .filter(item => {
      if (item.balance_amount <= 0) return false
      const key = amountKey(item.balance_amount)
      if (seen.has(key)) return false
      seen.add(key)
      return true
    })
    .sort((a, b) => a.balance_amount - b.balance_amount)
}

function defaultCostCalculatorConfig(): CostCalculatorConfig {
  return {
    balance_exchange_rate: 1,
    upstream_cost_rate: 1,
    account_costs: [],
    balance_recharge_packages: []
  }
}

function hasDuplicateRechargePackage(items: CostCalculatorBalanceRechargePackage[]): boolean {
  const seen = new Set<number>()
  for (const item of items) {
    const key = amountKey(item.balance_amount)
    if (seen.has(key)) return true
    seen.add(key)
  }
  return false
}

function amountKey(value: unknown): number {
  return Math.round(toFinite(value) * 1_000_000)
}

function positiveOrDefault(value: unknown, fallback: number): number {
  const n = toFinite(value)
  return n > 0 ? n : fallback
}

function nonNegativeOrDefault(value: unknown, fallback: number): number {
  const n = toFinite(value)
  return n >= 0 ? n : fallback
}

function toFinite(value: unknown): number {
  const n = typeof value === 'number' ? value : Number(value)
  return Number.isFinite(n) ? n : 0
}

function formatActualMoney(value: unknown): string {
  const n = toFinite(value)
  const digits = Math.abs(n) >= 1000 ? 2 : 4
  return `¥${n.toFixed(digits)}`
}

function formatPlatformBalance(value: unknown): string {
  const n = toFinite(value)
  const digits = Math.abs(n) >= 1000 ? 2 : 4
  return `$${n.toFixed(digits)}`
}

function formatEstimatedBalanceLiability(): string {
  if (!hasBalanceLiabilityValuation.value) {
    return '-'
  }
  return `${formatActualMoney(balanceLiabilityEstimatedActual.value)} (${formatActualMoney(balanceLiabilityEstimatedUnitCost.value)}/余额)`
}

function formatCompositeUsageRate(value: unknown): string {
  const n = nonNegativeOrDefault(value, 1)
  return `¥${n.toFixed(4).replace(/\.?0+$/, '')}/USD`
}

function formatRmbPerUsd(value: unknown): string {
  const n = positiveOrDefault(value, 1)
  return `¥${n.toFixed(4).replace(/\.?0+$/, '')}/USD`
}

function formatPercent(value: number): string {
  const n = Number.isFinite(value) ? value : 0
  return `${(n * 100).toFixed(2)}%`
}

function defaultStartDate(): string {
  const d = new Date()
  d.setDate(d.getDate() - 29)
  return formatDate(d)
}

function defaultEndDate(): string {
  return formatDate(new Date())
}

function formatDate(date: Date): string {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}
</script>
