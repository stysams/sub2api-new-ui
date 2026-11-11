<template>
  <AppLayout>
    <div class="space-y-5">
      <header>
        <h1 class="text-xl font-semibold text-gray-900 dark:text-white">{{ t('nav.modelFingerprint') }}</h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.modelFingerprint.subtitle') }}</p>
      </header>
      <p v-if="error" role="alert" class="rounded-lg bg-red-50 p-3 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300">{{ error }}</p>
      <section class="card flex flex-wrap items-center justify-between gap-3 p-4" :aria-busy="downloading">
        <div class="min-w-0 text-sm">
          <h2 class="font-medium">{{ t('admin.modelFingerprint.reference') }}</h2>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ reference.available ? reference.built_at : t('admin.modelFingerprint.noReference') }}</p>
          <p v-if="reference.sha256" class="mt-1 break-all font-mono text-xs text-gray-500">SHA-256: {{ reference.sha256 }}</p>
        </div>
        <button type="button" class="btn btn-secondary" :disabled="downloading || busy" @click="download">{{ t(downloading ? 'admin.modelFingerprint.downloading' : 'admin.modelFingerprint.download') }}</button>
      </section>
      <form class="card space-y-4 p-4" @submit.prevent="run">
        <div class="grid gap-4 md:grid-cols-3">
          <label class="input-label">{{ t('admin.modelFingerprint.account') }}
            <select v-model.number="accountId" class="input mt-1" :disabled="busy || loading" required>
              <option :value="0" disabled>{{ t('admin.modelFingerprint.chooseAccount') }}</option>
              <option v-for="a in eligibleAccounts" :key="a.id" :value="a.id">{{ a.name }} · {{ a.platform }} · #{{ a.id }}</option>
            </select>
          </label>
          <label class="input-label">{{ t('admin.modelFingerprint.model') }}
            <input v-model="model" class="input mt-1" list="fingerprint-models" maxlength="200" required :disabled="busy" :placeholder="t('admin.modelFingerprint.modelPlaceholder')">
            <datalist id="fingerprint-models"><option v-for="m in modelOptions" :key="m" :value="m" /></datalist>
          </label>
          <label class="input-label">{{ t('admin.modelFingerprint.protocol') }}
            <select v-model="protocol" class="input mt-1" :disabled="busy"><option value="auto">{{ t('admin.modelFingerprint.auto') }}</option><option value="chat_completions">Chat Completions</option><option value="responses">Responses</option><option value="messages">Anthropic Messages</option></select>
          </label>
        </div>
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.modelFingerprint.scope') }}</p>
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.modelFingerprint.disclaimer') }}</p>
        <div class="flex gap-2">
          <button type="submit" class="btn btn-primary" :disabled="busy || loading || !reference.available || !accountId || !model.trim()">{{ t(busy ? 'admin.modelFingerprint.running' : 'admin.modelFingerprint.verify') }}</button>
          <button v-if="busy" type="button" class="btn btn-secondary" @click="controller?.abort()">{{ t('common.cancel') }}</button>
        </div>
      </form>
      <section v-if="latest" class="card space-y-4 p-4">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <h2 class="font-medium">{{ t('admin.modelFingerprint.latest') }} #{{ latest.id }} · {{ statusLabel(latest.status) }}</h2>
          <button type="button" class="btn btn-secondary" @click="exportRecord">{{ t('admin.modelFingerprint.export') }}</button>
        </div>
        <p class="break-all text-sm text-gray-500">{{ latest.account_name }} (#{{ latest.account_id }}) · {{ latest.model }} → {{ latest.upstream_model }} · {{ latest.endpoint }}</p>
        <template v-if="latest.analysis">
          <p class="text-sm">{{ t('admin.modelFingerprint.evidence', { count: latest.analysis.used_outputs, version: latest.analysis.bank_version }) }}</p>
          <p class="text-xs text-gray-500">{{ t(`admin.modelFingerprint.reason.${latest.analysis.reason}`) }}</p>
          <p class="break-all font-mono text-xs text-gray-500">SHA-256: {{ latest.analysis.detector_sha256 }}</p>
          <div v-if="latest.analysis.results.length" class="overflow-x-auto">
            <table class="w-full text-left text-sm">
              <thead class="border-b dark:border-dark-700"><tr><th class="py-2">{{ t('admin.modelFingerprint.candidate') }}</th><th>{{ t('admin.modelFingerprint.ranking') }}</th><th>{{ t('admin.modelFingerprint.probability') }}</th></tr></thead>
              <tbody class="divide-y dark:divide-dark-700"><tr v-for="candidate in latest.analysis.results.slice(0, 8)" :key="candidate.model"><td class="py-2">{{ candidate.display_name }} <span class="text-xs text-gray-500">{{ candidate.model }}</span></td><td>{{ candidate.score.toFixed(3) }}</td><td>{{ candidate.probability == null ? '—' : `${(candidate.probability * 100).toFixed(1)}%` }}</td></tr></tbody>
            </table>
          </div>
        </template>
        <details v-for="(sample, i) in latest.samples" :key="i" class="border-t pt-3 dark:border-dark-700">
          <summary class="cursor-pointer text-sm">{{ t('admin.modelFingerprint.sample', { index: i + 1 }) }} · {{ sample.count }}/{{ sample.expected_count }} · HTTP {{ sample.status_code || '—' }} · {{ sample.response_model || '—' }} · {{ sample.latency_ms }}ms</summary>
          <p class="my-2 text-xs text-gray-500">{{ sample.prompt }}</p>
          <pre v-if="sample.answer" class="max-h-64 overflow-auto whitespace-pre-wrap break-words rounded bg-gray-50 p-3 text-xs dark:bg-dark-900">{{ sample.answer }}</pre>
          <p v-if="sample.error" class="mt-2 text-sm text-red-600 dark:text-red-400">{{ sample.error }}</p>
        </details>
      </section>
      <section class="card overflow-hidden">
        <div class="flex flex-wrap items-center justify-between gap-3 p-4"><h2 class="font-medium">{{ t('admin.modelFingerprint.history') }} ({{ total }})</h2><button type="button" class="btn btn-secondary" :disabled="historyLoading" @click="refresh">{{ t('common.refresh') }}</button></div>
        <form class="flex flex-wrap gap-2 px-4 pb-4" @submit.prevent="filterHistory">
          <label class="sr-only" for="fingerprint-filter-account">{{ t('admin.modelFingerprint.account') }}</label><input id="fingerprint-filter-account" v-model.number="filterAccount" type="number" min="0" class="input w-40" :placeholder="t('admin.modelFingerprint.accountId')">
          <label class="sr-only" for="fingerprint-filter-model">{{ t('admin.modelFingerprint.model') }}</label><input id="fingerprint-filter-model" v-model="filterModel" class="input w-48" :placeholder="t('admin.modelFingerprint.model')">
          <button type="submit" class="btn btn-secondary">{{ t('common.search') }}</button>
        </form>
        <div class="divide-y border-t dark:divide-dark-700 dark:border-dark-700" :aria-busy="historyLoading">
          <button v-for="row in records" :key="row.id" type="button" class="grid w-full gap-1 px-4 py-3 text-left hover:bg-gray-50 dark:hover:bg-dark-700 md:grid-cols-[1fr_1fr_auto] md:items-center" @click="latest = row">
            <span class="truncate text-sm">#{{ row.id }} · {{ row.account_name }} (#{{ row.account_id }}) · {{ row.model }}</span><time class="text-xs text-gray-500">{{ new Date(row.created_at).toLocaleString() }}</time><span class="text-sm">{{ statusLabel(row.status) }}</span>
          </button>
          <p v-if="!records.length && !historyLoading" class="py-6 text-center text-sm text-gray-500">{{ t('admin.modelFingerprint.empty') }}</p>
        </div>
        <Pagination v-if="total > 0" :total="total" :page="page" :page-size="20" :show-page-size-selector="false" @update:page="changePage" />
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import { getReference, listAccounts, listFingerprintRecords, refreshReference, verifyFingerprint, type AccountOption, type FingerprintRecord, type Reference } from './api'
const { t } = useI18n()
const accounts = ref<AccountOption[]>([])
const accountId = ref(0)
const model = ref('')
const protocol = ref('auto')
const busy = ref(false)
const loading = ref(true)
const downloading = ref(false)
const historyLoading = ref(false)
const error = ref('')
const latest = ref<FingerprintRecord | null>(null)
const records = ref<FingerprintRecord[]>([])
const reference = ref<Reference>({ available: false })
const total = ref(0)
const page = ref(1)
const filterAccount = ref<number | ''>('')
const filterModel = ref('')
let controller: AbortController | undefined
const eligibleAccounts = computed(() => accounts.value.filter(a => !a.parent_account_id && !['gemini', 'antigravity', 'typesafe'].includes(a.platform)))
const modelOptions = computed(() => Object.keys(accounts.value.find(a => a.id === accountId.value)?.credentials?.model_mapping ?? {}).filter(m => !m.includes('*')))
const statusLabel = (status: string) => t(`admin.modelFingerprint.status.${status}`)
function showError(e: unknown) { const err = e as { message?: string }; error.value = err.message || t('admin.modelFingerprint.error') }
async function refresh() { historyLoading.value = true; try { const data = await listFingerprintRecords({ page: page.value, account_id: Number(filterAccount.value) || undefined, model: filterModel.value.trim() || undefined }); records.value = data.items; total.value = data.total } catch (e) { showError(e) } finally { historyLoading.value = false } }
async function run() { busy.value = true; error.value = ''; controller = new AbortController(); try { latest.value = await verifyFingerprint(accountId.value, model.value, protocol.value, controller.signal) } catch (e) { showError(e) } finally { busy.value = false; controller = undefined; page.value = 1; await refresh() } }
async function download() { downloading.value = true; error.value = ''; try { reference.value = await refreshReference() } catch (e) { showError(e) } finally { downloading.value = false } }
function filterHistory() { page.value = 1; void refresh() }
function changePage(value: number) { page.value = value; void refresh() }
function exportRecord() { if (!latest.value) return; const url = URL.createObjectURL(new Blob([JSON.stringify(latest.value, null, 2)], { type: 'application/json' })); const anchor = document.createElement('a'); anchor.href = url; anchor.download = `model-fingerprint-${latest.value.id}.json`; anchor.click(); URL.revokeObjectURL(url) }
onMounted(async () => { await Promise.all([refresh(), (async () => { try { reference.value = await getReference() } catch (e) { showError(e) } })(), (async () => { try { accounts.value = await listAccounts() } catch (e) { showError(e) } finally { loading.value = false } })()]) })
onBeforeUnmount(() => controller?.abort())
</script>
