export interface UpstreamGroupItem {
  id?: string
  name: string
  ratio: number | null
  desc?: string
  platform?: string
}

export interface UpstreamBalanceSnapshot {
  quota?: number
  used_quota?: number
  remaining?: number
  request_count?: number
  currency?: string
  fetched_at?: string
  [key: string]: unknown
}

export interface Upstream {
  id: number
  name: string
  sort_code: number
  kind: 'newapi' | 'sub2api' | string
  base_url: string
  token_expires_at?: string | null
  login_identifier?: string
  remote_user_id?: string
  balance_snapshot: UpstreamBalanceSnapshot
  group_snapshot: { items?: UpstreamGroupItem[]; fetched_at?: string }
  notes?: string
  enabled: boolean
  last_checked_at?: string | null
  last_error?: string
  created_at: string
  updated_at: string
  token_masked?: string
}

export interface UpstreamResource {
  id: number
  upstream_id: number
  resource_type: string
  remote_id: string
  name: string
  group_name?: string
  models_snapshot: string[]
  models_fetched_at?: string | null
  synced_account_id?: number | null
  synced_rate_multiplier?: number | null
  synced_at?: string | null
  enabled: boolean
  key_masked: string
}

export interface UpstreamInput {
  name: string
  sort_code?: number
  kind: string
  base_url: string
  token?: string
  login_identifier?: string
  remote_user_id?: string
  password?: string
  notes?: string
  enabled?: boolean
}

export interface UpstreamSyncResult {
  created: number
  updated: number
  skipped: number
  failed: number
  items: Array<{ resource_id: number; status: string; account_id?: number; message?: string }>
}
