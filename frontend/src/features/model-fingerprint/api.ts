import api from '@/api/client'

export interface FingerprintSample {
  prompt: string; expected_count: number; count: number; status_code: number
  response_model?: string; response_id?: string; answer?: string; error?: string; latency_ms: number
}
export interface Candidate { model: string; display_name: string; family: string; score: number; probability: number | null }
export interface Analysis {
  decision: string; prediction: string; used_outputs: number; results: Candidate[]
  method: string; bank_version: string; detector_sha256: string; reason: string
}
export interface FingerprintRecord {
  id: number; account_id: number; account_name: string; platform: string; model: string
  upstream_model: string; endpoint: string; status: string; score: number
  samples: FingerprintSample[]; analysis: Analysis | null; created_at: string
}
export interface Reference { available: boolean; built_at?: string; sha256?: string }
export interface RecordFilter { page: number; account_id?: number; model?: string }
export async function verifyFingerprint(account_id: number, model: string, protocol = 'auto', signal?: AbortSignal) {
  const { data } = await api.post<FingerprintRecord>('/admin/model-fingerprint/verify', { account_id, model, api: protocol }, { timeout: 390000, signal })
  return data
}
export async function listFingerprintRecords(filter: RecordFilter) {
  const { data } = await api.get<{ items: FingerprintRecord[]; total: number }>('/admin/model-fingerprint/records', { params: { ...filter, page_size: 20 } })
  return data
}
export async function getReference() { return (await api.get<Reference>('/admin/model-fingerprint/reference')).data }
export async function refreshReference() { return (await api.post<Reference>('/admin/model-fingerprint/reference', {}, { timeout: 200000 })).data }
export interface AccountOption { id: number; name: string; platform: string; type: string; parent_account_id?: number; credentials?: { model_mapping?: Record<string, string> } }
export async function listAccounts() {
  const accounts: AccountOption[] = []
  for (let page = 1; ; page++) {
    const { data } = await api.get<{ items: AccountOption[]; total: number }>('/admin/accounts', { params: { page, page_size: 100, type: 'apikey', lite: true } })
    accounts.push(...data.items)
    if (!data.items.length || accounts.length >= data.total) return accounts
  }
}
