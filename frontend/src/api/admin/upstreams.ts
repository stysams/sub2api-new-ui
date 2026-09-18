import { apiClient } from '../client'
import type {
  Upstream,
  UpstreamGroupItem,
  UpstreamInput,
  UpstreamResource,
  UpstreamSyncResult
} from '@/types'

export interface UpstreamListParams {
  page?: number
  page_size?: number
  search?: string
}

export interface PaginatedResponse<T> {
  items: T[]
  total: number
  page: number
  page_size: number
  pages: number
}

export interface UpstreamBalanceNotifySettings {
  enabled: boolean
  threshold: number
  emails: string[]
}

export async function listPaginated(params: UpstreamListParams = {}): Promise<PaginatedResponse<Upstream>> {
  const { data } = await apiClient.get<PaginatedResponse<Upstream>>('/admin/upstreams', { params })
  return data
}

export async function list(): Promise<Upstream[]> {
  const { data } = await apiClient.get<Upstream[]>('/admin/upstreams')
  return data
}

export async function getById(id: number): Promise<Upstream> {
  const { data } = await apiClient.get<Upstream>(`/admin/upstreams/${id}`)
  return data
}

export async function create(input: UpstreamInput): Promise<Upstream> {
  const { data } = await apiClient.post<Upstream>('/admin/upstreams', input)
  return data
}

export async function update(id: number, input: UpstreamInput): Promise<Upstream> {
  const { data } = await apiClient.put<Upstream>(`/admin/upstreams/${id}`, input)
  return data
}

export async function remove(id: number): Promise<void> {
  await apiClient.delete(`/admin/upstreams/${id}`)
}

export async function test(id: number): Promise<Upstream> {
  const { data } = await apiClient.post<Upstream>(`/admin/upstreams/${id}/test`)
  return data
}

export async function refreshBalance(id: number): Promise<Upstream> {
  const { data } = await apiClient.post<Upstream>(`/admin/upstreams/${id}/balance/refresh`)
  return data
}

export async function getBalanceNotifySettings(): Promise<UpstreamBalanceNotifySettings> {
  const { data } = await apiClient.get<UpstreamBalanceNotifySettings>('/admin/upstreams/balance-notify-settings')
  return data
}

export async function updateBalanceNotifySettings(input: UpstreamBalanceNotifySettings): Promise<UpstreamBalanceNotifySettings> {
  const { data } = await apiClient.put<UpstreamBalanceNotifySettings>('/admin/upstreams/balance-notify-settings', input)
  return data
}

export async function refreshAllBalances(): Promise<Upstream[]> {
  const { data } = await apiClient.post<Upstream[]>('/admin/upstreams/balance/refresh-all')
  return data
}

export async function groups(id: number, refresh = false): Promise<UpstreamGroupItem[]> {
  const { data } = await apiClient.get<UpstreamGroupItem[]>(`/admin/upstreams/${id}/groups`, {
    params: refresh ? { refresh: true } : undefined
  })
  return data
}

export async function resources(id: number): Promise<UpstreamResource[]> {
  const { data } = await apiClient.get<UpstreamResource[]>(`/admin/upstreams/${id}/resources`)
  return data
}

export async function createKey(id: number, input: { group: string; name: string }): Promise<UpstreamResource> {
  const { data } = await apiClient.post<UpstreamResource>(`/admin/upstreams/${id}/keys`, input)
  return data
}

export async function refreshModels(resourceId: number): Promise<string[]> {
  const { data } = await apiClient.post<string[]>(`/admin/upstreams/resources/${resourceId}/models/refresh`)
  return data
}

export async function syncToAccount(resourceId: number, groupIds: number[]): Promise<UpstreamSyncResult> {
  const { data } = await apiClient.post<UpstreamSyncResult>(`/admin/upstreams/resources/${resourceId}/sync`, {
    group_ids: groupIds
  })
  return data
}

export default {
  list,
  listPaginated,
  getById,
  create,
  update,
  remove,
  test,
  refreshBalance,
  getBalanceNotifySettings,
  updateBalanceNotifySettings,
  refreshAllBalances,
  groups,
  resources,
  createKey,
  refreshModels,
  syncToAccount
}
