<template>
  <AppLayout>
    <div class="mx-auto max-w-7xl space-y-3 p-4 sm:p-5 lg:space-y-4">
      <div class="flex items-center justify-end gap-2">
        <button
          type="button"
          class="btn btn-secondary flex h-11 w-11 shrink-0 items-center justify-center p-0"
          :disabled="loading"
          :title="t('common.refresh')"
          :aria-label="t('common.refresh')"
          @click="refreshUpstreams"
        >
          <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
        </button>
        <div class="relative min-w-0 flex-1 sm:flex-none">
          <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
          <input
            v-model="searchQuery"
            type="text"
            class="input w-full pl-9 sm:w-64"
            :placeholder="t('admin.upstreams.searchPlaceholder')"
            @input="onSearchInput"
          />
        </div>
        <button type="button" class="btn btn-primary shrink-0" @click="openCreate">
          <Icon name="plus" size="sm" class="mr-1.5" />
          {{ t('admin.upstreams.add') }}
        </button>
      </div>

      <section
        class="card overflow-hidden border border-primary-100/80 bg-gradient-to-r from-primary-50/80 via-white to-blue-50/50 p-3 dark:border-primary-900/40 dark:from-primary-950/30 dark:via-dark-800/50 dark:to-blue-950/20 sm:px-4"
      >
        <div class="flex flex-col gap-2 xl:flex-row xl:items-center xl:justify-between">
          <div class="flex min-w-0 flex-1 items-center gap-3">
            <span
              class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary-100 text-primary-600 dark:bg-primary-900/40 dark:text-primary-400"
            >
              <Icon name="bell" size="sm" />
            </span>
            <div class="min-w-0">
              <h2 class="text-sm font-semibold text-gray-900 dark:text-white">
                {{ t('admin.upstreams.balanceNotifyTitle') }}
              </h2>
              <p class="truncate text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.upstreams.balanceNotifyHint') }}
              </p>
            </div>
            <button
              type="button"
              role="switch"
              :aria-checked="balanceSettings.enabled"
              :class="['switch ml-1 shrink-0', balanceSettings.enabled ? 'switch-active' : '']"
              :title="t('admin.upstreams.balanceNotifyEnabled')"
              :aria-label="t('admin.upstreams.balanceNotifyEnabled')"
              @click="balanceSettings.enabled = !balanceSettings.enabled"
            >
              <span class="switch-thumb" />
            </button>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <label
              class="flex items-center gap-2 rounded-lg bg-white/70 px-2.5 py-1.5 text-xs text-gray-500 ring-1 ring-gray-200 dark:bg-dark-800/60 dark:text-gray-400 dark:ring-dark-600"
            >
              {{ t('admin.upstreams.balanceThreshold') }}
              <input
                v-model.number="balanceSettings.threshold"
                type="number"
                min="0"
                max="100"
                step="0.01"
                class="w-16 bg-transparent text-right text-sm font-medium text-gray-800 focus:outline-none dark:text-gray-100"
              />
            </label>
            <button type="button" class="btn btn-secondary btn-sm" :disabled="refreshAllBusy" @click="refreshAllBalances">
              <Icon name="refresh" size="sm" class="mr-1" :class="refreshAllBusy ? 'animate-spin' : ''" />
              {{ t('admin.upstreams.refreshAllBalances') }}
            </button>
            <button type="button" class="btn btn-primary btn-sm" :disabled="balanceSettingsSaving" @click="saveBalanceSettings">
              {{ balanceSettingsSaving ? t('common.saving') : t('admin.upstreams.saveBalanceSettings') }}
            </button>
          </div>
        </div>
        <div class="mt-2 flex flex-wrap items-center gap-1.5 border-t border-primary-100/70 pt-2 dark:border-primary-900/30">
          <Icon name="mail" size="xs" class="mr-0.5 shrink-0 text-gray-400" />
          <span
            v-for="(email, index) in balanceSettings.emails"
            :key="index"
            class="inline-flex items-center gap-1 rounded-full bg-white py-1 pl-2.5 pr-1.5 text-xs text-gray-700 ring-1 ring-primary-100 dark:bg-dark-800 dark:text-gray-300 dark:ring-primary-900/40"
          >
            {{ email }}
            <button
              type="button"
              class="flex h-4 w-4 items-center justify-center rounded-full text-gray-400 transition-colors hover:bg-red-50 hover:text-red-500 dark:hover:bg-red-900/30"
              :title="t('admin.upstreams.removeEmail')"
              :aria-label="t('admin.upstreams.removeEmail')"
              @click="removeNotifyEmail(index)"
            >
              <Icon name="x" size="xs" />
            </button>
          </span>
          <input
            v-model="newNotifyEmail"
            type="email"
            class="h-7 w-44 rounded-full border border-dashed border-gray-300 bg-transparent px-2.5 text-xs text-gray-700 transition-colors placeholder:text-gray-400 focus:border-primary-400 focus:outline-none dark:border-dark-600 dark:text-gray-200 dark:focus:border-primary-500"
            :placeholder="t('admin.upstreams.emailPlaceholder')"
            @keydown.enter.prevent="addNotifyEmailFromInput"
          />
        </div>
      </section>

      <div v-if="!loading && upstreams.length" class="grid grid-cols-3 gap-2 sm:gap-3">
        <div class="stat-card card-hover items-center gap-2 p-2.5 sm:gap-3 sm:p-3">
          <div class="stat-icon hidden h-8 w-8 shrink-0 rounded-lg bg-gradient-to-br from-primary-500 to-blue-600 text-white shadow-md shadow-primary-500/20 sm:flex sm:h-9 sm:w-9">
            <Icon name="cloud" size="lg" />
          </div>
          <div class="min-w-0">
            <span class="stat-label text-xs">{{ t('admin.upstreams.total') }}</span>
            <strong class="stat-value block text-lg leading-tight">{{ totalUpstreams }}</strong>
          </div>
        </div>
        <div class="stat-card card-hover items-center gap-2 p-2.5 sm:gap-3 sm:p-3">
          <div class="stat-icon hidden h-8 w-8 shrink-0 rounded-lg bg-gradient-to-br from-emerald-500 to-teal-600 text-white shadow-md shadow-emerald-500/20 sm:flex sm:h-9 sm:w-9">
            <Icon name="bolt" size="lg" />
          </div>
          <div class="min-w-0">
            <span class="stat-label text-xs">{{ t('admin.upstreams.active') }}</span>
            <strong class="stat-value block text-lg leading-tight text-emerald-600 dark:text-emerald-400">{{ activeUpstreamCount }}</strong>
          </div>
        </div>
        <div class="stat-card card-hover items-center gap-2 p-2.5 sm:gap-3 sm:p-3">
          <div class="stat-icon hidden h-8 w-8 shrink-0 rounded-lg bg-gradient-to-br from-violet-500 to-purple-600 text-white shadow-md shadow-violet-500/20 sm:flex sm:h-9 sm:w-9">
            <Icon name="key" size="lg" />
          </div>
          <div class="min-w-0">
            <span class="stat-label text-xs">{{ t('admin.upstreams.keyCount') }}</span>
            <strong class="stat-value block text-lg leading-tight text-violet-600 dark:text-violet-400">{{ resourceCount }}</strong>
          </div>
        </div>
      </div>

      <!-- Loading State -->
      <div
        v-if="loading"
        class="rounded-lg border border-gray-200 bg-white py-16 text-center text-sm text-gray-500 shadow-sm dark:border-dark-700 dark:bg-dark-800 dark:text-gray-400"
      >
        <Icon name="refresh" size="md" class="mr-2 animate-spin" />
        {{ t('common.loading') }}
      </div>

      <!-- Empty State -->
      <div
        v-else-if="upstreams.length === 0"
        class="card border border-dashed border-gray-300 py-16 text-center shadow-sm dark:border-dark-600"
      >
        <span
          class="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-2xl bg-gradient-to-br from-primary-100 to-blue-100 text-primary-500 dark:from-primary-900/30 dark:to-blue-900/20 dark:text-primary-400"
        >
          <Icon name="cloud" size="xl" />
        </span>
        <p class="text-sm font-medium text-gray-700 dark:text-gray-300">
          {{ t('admin.upstreams.empty') }}
        </p>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.upstreams.emptyHint') }}
        </p>
        <button type="button" class="btn btn-primary mt-4" @click="openCreate">
          <Icon name="plus" size="sm" class="mr-1.5" />
          {{ t('admin.upstreams.add') }}
        </button>
      </div>

      <!-- Upstream List -->
      <div v-else class="space-y-3">
        <section
          v-for="upstream in upstreams"
          :key="upstream.id"
          class="card card-hover overflow-hidden p-0"
        >
          <!-- Card Header -->
          <div class="flex flex-col gap-3 p-4 sm:p-5 xl:flex-row xl:items-center xl:justify-between">
            <button
              type="button"
              class="flex min-w-0 flex-1 items-center gap-3.5 text-left"
              :aria-expanded="expanded.has(upstream.id)"
              :aria-label="`${expanded.has(upstream.id) ? t('admin.upstreams.collapse') : t('admin.upstreams.expand')} ${upstream.name}`"
              @click="toggleExpanded(upstream)"
            >
              <span class="relative shrink-0">
                <span
                  :class="[
                    'flex h-11 w-11 items-center justify-center rounded-xl text-white shadow-lg',
                    kindTileClass(upstream.kind)
                  ]"
                >
                  <Icon :name="upstream.kind === 'sub2api' ? 'cloud' : 'server'" size="md" />
                </span>
                <span
                  :class="[
                    'absolute -right-0.5 -top-0.5 h-3 w-3 rounded-full ring-2 ring-white dark:ring-dark-800',
                    upstream.enabled ? 'bg-emerald-500' : 'bg-gray-300 dark:bg-dark-500'
                  ]"
                />
              </span>
              <span class="min-w-0">
                <span class="flex flex-wrap items-center gap-x-2 gap-y-1">
                  <span class="truncate text-[15px] font-semibold text-gray-900 dark:text-white">
                    {{ upstream.name }}
                  </span>
                  <span :class="['badge', kindBadgeClass(upstream.kind)]">{{ kindLabel(upstream.kind) }}</span>
                  <span :class="['badge', upstream.enabled ? 'badge-success' : 'badge-gray']">
                    {{ upstream.enabled ? t('common.enabled') : t('common.disabled') }}
                  </span>
                  <span v-if="upstream.resource_count" class="badge badge-gray">
                    <Icon name="key" size="xs" />
                    {{ upstream.resource_count }}
                  </span>
                </span>
                <span class="mt-1 flex items-center gap-1.5 text-xs text-gray-500 dark:text-gray-400">
                  <Icon name="link" size="xs" class="shrink-0 text-gray-400" />
                  <span class="truncate font-mono">{{ upstream.base_url }}</span>
                </span>
              </span>
              <Icon
                name="chevronDown"
                size="md"
                class="ml-auto shrink-0 text-gray-300 transition-transform duration-200 dark:text-dark-500"
                :class="expanded.has(upstream.id) ? 'rotate-180' : ''"
              />
            </button>

            <!-- Card Actions -->
            <div class="flex flex-wrap items-center gap-2 xl:justify-end">
              <div
                class="flex items-center gap-2.5 rounded-xl bg-gradient-to-r from-emerald-50 to-teal-50 py-1.5 pl-3 pr-2 ring-1 ring-emerald-100 dark:from-emerald-950/40 dark:to-teal-950/30 dark:ring-emerald-900/40"
                :title="upstream.balance_snapshot?.fetched_at ? dateLabel(upstream.balance_snapshot.fetched_at) : undefined"
              >
                <div class="leading-tight">
                  <div class="flex items-center gap-1 text-[11px] font-medium text-emerald-600/80 dark:text-emerald-400/70">
                    <Icon name="clock" size="xs" />
                    {{ freshnessLabel(upstream) }}
                  </div>
                  <div class="mt-0.5 flex items-baseline gap-1 whitespace-nowrap">
                    <span class="text-[11px] text-emerald-600/70 dark:text-emerald-400/60">{{ t('admin.upstreams.remaining') }}</span>
                    <strong class="text-lg font-bold tabular-nums text-emerald-700 dark:text-emerald-300">
                      {{ remainingLabel(upstream) }}
                    </strong>
                    <span class="text-[11px] font-medium text-emerald-600/70 dark:text-emerald-400/60">{{ currencyLabel(upstream) }}</span>
                  </div>
                </div>
                <button
                  type="button"
                  class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-emerald-600/70 transition-colors hover:bg-white hover:text-emerald-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-500/30 disabled:cursor-not-allowed disabled:opacity-50 dark:hover:bg-dark-800 dark:hover:text-emerald-400"
                  :disabled="balanceBusyId === upstream.id"
                  :title="t('admin.upstreams.refreshBalance')"
                  :aria-label="t('admin.upstreams.refreshBalance')"
                  @click.stop="refreshBalance(upstream)"
                >
                  <Icon name="refresh" size="sm" :class="balanceBusyId === upstream.id ? 'animate-spin' : ''" />
                </button>
              </div>
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="busyId === upstream.id"
                @click="testConnection(upstream)"
              >
                <Icon
                  :name="busyId === upstream.id ? 'refresh' : 'play'"
                  size="sm"
                  class="mr-1"
                  :class="busyId === upstream.id ? 'animate-spin' : ''"
                />
                {{ busyId === upstream.id ? t('admin.upstreams.testing') : t('admin.upstreams.test') }}
              </button>
              <div class="flex items-center gap-1">
                <button
                  type="button"
                  class="flex h-8 w-8 items-center justify-center rounded-lg text-gray-400 transition-colors hover:bg-primary-50 hover:text-primary-600 dark:hover:bg-dark-700 dark:hover:text-primary-400"
                  :title="t('common.edit')"
                  :aria-label="t('common.edit')"
                  @click="openEdit(upstream)"
                >
                  <Icon name="edit" size="sm" />
                </button>
                <a
                  :href="upstream.base_url"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="flex h-8 w-8 items-center justify-center rounded-lg text-gray-400 transition-colors hover:bg-primary-50 hover:text-primary-600 dark:hover:bg-dark-700 dark:hover:text-primary-400"
                  :title="t('admin.upstreams.openSite')"
                  :aria-label="t('admin.upstreams.openSite')"
                >
                  <Icon name="externalLink" size="sm" />
                </a>
                <button
                  type="button"
                  class="flex h-8 w-8 items-center justify-center rounded-lg text-gray-400 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
                  :title="t('admin.upstreams.delete')"
                  :aria-label="t('admin.upstreams.delete')"
                  @click="confirmDeleteUpstream(upstream)"
                >
                  <Icon name="trash" size="sm" />
                </button>
              </div>
            </div>
          </div>

          <!-- Error Strip -->
          <div
            v-if="upstream.last_error"
            class="flex items-center gap-2 border-t border-red-100 bg-red-50/60 px-4 py-2 text-xs text-red-700 dark:border-red-900/30 dark:bg-red-950/20 dark:text-red-300 sm:px-5"
          >
            <Icon name="exclamationCircle" size="xs" class="shrink-0" />
            <span class="truncate" :title="upstream.last_error">{{ upstream.last_error }}</span>
          </div>

          <!-- Expanded Details -->
          <div
            v-if="expanded.has(upstream.id)"
            class="border-t border-gray-100 bg-gray-50/60 p-4 dark:border-dark-700 dark:bg-dark-900/30 sm:p-5"
          >
            <div class="grid gap-4 lg:grid-cols-2">
              <!-- Groups Panel -->
              <div class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
                <div class="mb-3 flex items-center justify-between gap-3">
                  <h2 class="flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
                    <span class="flex h-7 w-7 items-center justify-center rounded-lg bg-blue-100 text-blue-600 dark:bg-blue-900/30 dark:text-blue-400">
                      <Icon name="grid" size="xs" />
                    </span>
                    {{ t('admin.upstreams.groups') }}
                  </h2>
                  <button
                    type="button"
                    class="btn btn-ghost btn-sm"
                    :disabled="groupLoadingIds.has(upstream.id)"
                    @click="loadGroups(upstream, true)"
                  >
                    <Icon name="refresh" size="sm" class="mr-1" :class="groupLoadingIds.has(upstream.id) ? 'animate-spin' : ''" />
                    {{ t('admin.upstreams.refreshGroups') }}
                  </button>
                </div>

                <!-- Groups Table -->
                <div
                  v-if="!groupsByUpstream[upstream.id]?.length && groupLoadingIds.has(upstream.id)"
                  class="flex items-center justify-center py-5 text-sm text-gray-500 dark:text-gray-400"
                >
                  <Icon name="refresh" size="sm" class="mr-2 animate-spin" />
                  {{ t('common.loading') }}
                </div>
                <div
                  v-else-if="groupsByUpstream[upstream.id]?.length"
                  class="overflow-x-auto"
                >
                  <table class="w-full text-left text-sm">
                    <thead>
                      <tr class="border-b border-gray-200 text-xs uppercase text-gray-500 dark:border-dark-600 dark:text-gray-400">
                        <th class="pb-2 pr-3 font-medium">{{ t('admin.upstreams.group') }}</th>
                        <th class="pb-2 pr-3 font-medium">{{ t('admin.upstreams.ratio') }}</th>
                        <th class="pb-2 font-medium">{{ t('admin.upstreams.platformType') }}</th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr
                        v-for="group in groupsByUpstream[upstream.id]"
                        :key="`${upstream.id}-${group.name}`"
                        class="border-b border-gray-50 transition-colors last:border-0 hover:bg-gray-50/50 dark:border-dark-800 dark:hover:bg-dark-800/50"
                      >
                        <td class="py-2.5 pr-3 font-medium text-gray-800 dark:text-gray-200">
                          {{ group.name }}
                        </td>
                        <td class="py-2.5 pr-3">
                          <span class="badge badge-primary">
                            {{ ratioLabel(group.ratio) }}
                          </span>
                        </td>
                        <td class="py-2.5 text-xs text-gray-500">
                          <span class="badge badge-gray">{{ group.platform || '-' }}</span>
                        </td>
                      </tr>
                    </tbody>
                  </table>
                </div>
                <p
                  v-else
                  class="py-5 text-center text-sm text-gray-500 dark:text-gray-400"
                >
                  {{ t('admin.upstreams.noGroups') }}
                </p>

                <!-- Balance Summary -->
                <div
                  class="mt-4 grid grid-cols-3 gap-2 border-t border-gray-100 pt-4 dark:border-dark-700"
                >
                  <div class="rounded-lg bg-gray-50 px-3 py-2 text-center dark:bg-dark-900/50">
                    <span class="block text-xs text-gray-500 dark:text-gray-400">{{ t('admin.upstreams.quota') }}</span>
                    <strong class="mt-0.5 block text-sm font-semibold tabular-nums text-gray-800 dark:text-gray-200">
                      {{ numberLabel(upstream.balance_snapshot?.quota) }}
                    </strong>
                  </div>
                  <div class="rounded-lg bg-gray-50 px-3 py-2 text-center dark:bg-dark-900/50">
                    <span class="block text-xs text-gray-500 dark:text-gray-400">{{ t('admin.upstreams.usedQuota') }}</span>
                    <strong class="mt-0.5 block text-sm font-semibold tabular-nums text-gray-800 dark:text-gray-200">
                      {{ numberLabel(upstream.balance_snapshot?.used_quota) }}
                    </strong>
                  </div>
                  <div class="rounded-lg bg-gray-50 px-3 py-2 text-center dark:bg-dark-900/50">
                    <span class="block text-xs text-gray-500 dark:text-gray-400">{{ t('admin.upstreams.requestCount') }}</span>
                    <strong class="mt-0.5 block text-sm font-semibold tabular-nums text-gray-800 dark:text-gray-200">
                      {{ numberLabel(upstream.balance_snapshot?.request_count) }}
                    </strong>
                  </div>
                </div>
                <div class="mt-3 flex flex-wrap items-center justify-between gap-2 text-xs text-gray-500 dark:text-gray-400">
                  <span v-if="upstream.balance_snapshot?.currency" class="inline-flex items-center gap-1">
                    <Icon name="dollar" size="xs" />
                    {{ upstream.balance_snapshot.currency }}
                  </span>
                  <span v-if="upstream.balance_snapshot?.fetched_at" class="inline-flex items-center gap-1">
                    <Icon name="clock" size="xs" />
                    {{ t('admin.upstreams.lastFetched') }} {{ dateLabel(upstream.balance_snapshot.fetched_at) }}
                  </span>
                </div>
              </div>

              <!-- Resources Panel -->
              <div class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
                <div class="mb-3 flex items-center justify-between gap-3">
                  <h2 class="flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
                    <span class="flex h-7 w-7 items-center justify-center rounded-lg bg-violet-100 text-violet-600 dark:bg-violet-900/30 dark:text-violet-400">
                      <Icon name="key" size="xs" />
                    </span>
                    {{ t('admin.upstreams.resources') }}
                  </h2>
                  <button
                    type="button"
                    class="btn btn-primary btn-sm"
                    @click="openKey(upstream)"
                  >
                    <Icon name="key" size="sm" class="mr-1" />
                    {{ t('admin.upstreams.addKey') }}
                  </button>
                </div>

                <!-- Resource List -->
                <div
                  v-if="resourcesByUpstream[upstream.id]?.length"
                  class="space-y-2.5"
                >
                  <div
                    v-for="resource in resourcesByUpstream[upstream.id]"
                    :key="resource.id"
                    class="rounded-xl border border-gray-100 p-3 transition-all hover:border-primary-200 hover:shadow-sm dark:border-dark-700 dark:hover:border-dark-500"
                  >
                    <div class="flex items-start justify-between gap-3">
                      <div class="min-w-0">
                        <div class="flex flex-wrap items-center gap-2">
                          <Icon name="key" size="xs" class="shrink-0 text-gray-400" />
                          <span class="font-medium text-gray-800 dark:text-gray-200">
                            {{ resource.name }}
                          </span>
                          <code class="rounded bg-gray-100 px-1.5 py-0.5 text-xs text-gray-500 dark:bg-dark-700">
                            {{ resource.key_masked }}
                          </code>
                        </div>
                        <div class="mt-1.5 flex flex-wrap items-center gap-2 text-xs text-gray-500">
                          <span v-if="resource.group_name">{{ resource.group_name }}</span>
                          <span v-if="resource.models_snapshot?.length" class="inline-flex items-center gap-1">
                            <Icon name="cube" size="xs" class="text-gray-400" />
                            {{ resource.models_snapshot.length }} {{ t('admin.upstreams.models') }}
                          </span>
                          <span
                            :class="[
                              'inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium',
                              resource.synced_account_id
                                ? 'bg-green-50 text-green-700 dark:bg-green-900/20 dark:text-green-400'
                                : 'bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-400'
                            ]"
                          >
                            <Icon :name="resource.synced_account_id ? 'checkCircle' : 'exclamationCircle'" size="xs" />
                            {{
                              resource.synced_account_id
                                ? `${t('admin.upstreams.synced')} #${resource.synced_account_id}`
                                : t('admin.upstreams.unsynced')
                            }}
                          </span>
                        </div>
                      </div>

                      <!-- Resource Actions -->
                      <div class="flex shrink-0 gap-1">
                        <button
                          type="button"
                          class="flex h-8 w-8 items-center justify-center rounded-lg text-gray-400 transition-colors hover:bg-primary-50 hover:text-primary-600 dark:hover:bg-dark-700 dark:hover:text-primary-400"
                          :title="t('admin.upstreams.refreshModels')"
                          :aria-label="t('admin.upstreams.refreshModels')"
                          @click="refreshResourceModels(resource)"
                        >
                          <Icon name="refresh" size="sm" />
                        </button>
                        <button
                          type="button"
                          class="flex h-8 w-8 items-center justify-center rounded-lg text-gray-400 transition-colors hover:bg-primary-50 hover:text-primary-600 dark:hover:bg-dark-700 dark:hover:text-primary-400"
                          :title="t('admin.upstreams.chat')"
                          :aria-label="t('admin.upstreams.chat')"
                          @click="openChat(resource)"
                        >
                          <Icon name="chatBubble" size="sm" />
                        </button>
                        <button
                          type="button"
                          class="flex h-8 w-8 items-center justify-center rounded-lg text-gray-400 transition-colors hover:bg-primary-50 hover:text-primary-600 dark:hover:bg-dark-700 dark:hover:text-primary-400"
                          :title="t('admin.upstreams.sync')"
                          :aria-label="t('admin.upstreams.sync')"
                          @click="openSync(resource)"
                        >
                          <Icon name="upload" size="sm" />
                        </button>
                      </div>
                    </div>

                    <!-- Model Tags -->
                    <div
                      v-if="resource.models_snapshot?.length"
                      class="mt-2.5 flex flex-wrap gap-1.5"
                    >
                      <span
                        v-for="m in resource.models_snapshot.slice(0, 8)"
                        :key="m"
                        class="rounded-md bg-primary-50/80 px-2 py-0.5 text-xs text-primary-700 ring-1 ring-primary-100 dark:bg-primary-900/20 dark:text-primary-300 dark:ring-primary-900/40"
                      >
                        {{ m }}
                      </span>
                      <span
                        v-if="resource.models_snapshot.length > 8"
                        class="rounded-md bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-500 dark:bg-dark-700 dark:text-gray-400"
                      >
                        +{{ resource.models_snapshot.length - 8 }}
                      </span>
                    </div>
                    <div
                      v-else
                      class="mt-2.5 flex items-center gap-1.5 text-xs text-gray-400"
                    >
                      <Icon name="exclamationCircle" size="xs" />
                      {{ t('admin.upstreams.noModels') }}
                    </div>
                  </div>
                </div>

                <div
                  v-else-if="resourceLoadingIds.has(upstream.id)"
                  class="flex items-center justify-center py-5 text-sm text-gray-500 dark:text-gray-400"
                >
                  <Icon name="refresh" size="sm" class="mr-2 animate-spin" />
                  {{ t('common.loading') }}
                </div>
                <p
                  v-else
                  class="py-5 text-center text-sm text-gray-500 dark:text-gray-400"
                >
                  {{ t('admin.upstreams.noResources') }}
                </p>
              </div>
            </div>
          </div>
        </section>
      </div>

      <!-- Pagination -->
      <div
        v-if="!loading && totalPages > 1"
        class="flex items-center justify-between rounded-lg border border-gray-200 bg-white px-4 py-3 shadow-sm dark:border-dark-700 dark:bg-dark-800"
      >
        <span class="text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.upstreams.totalCount', { count: totalUpstreams }) }}
        </span>
        <div class="flex items-center gap-2">
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            :disabled="currentPage <= 1"
            @click="goToPage(currentPage - 1)"
          >
            {{ t('admin.upstreams.prevPage') }}
          </button>
          <span class="text-sm text-gray-700 dark:text-gray-300">
            {{ t('admin.upstreams.pageInfo', { page: currentPage, pages: totalPages }) }}
          </span>
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            :disabled="currentPage >= totalPages"
            @click="goToPage(currentPage + 1)"
          >
            {{ t('admin.upstreams.nextPage') }}
          </button>
        </div>
      </div>
    </div>

    <!-- Create / Edit Dialog -->
    <BaseDialog
      :show="showForm"
      :title="editingId ? t('admin.upstreams.edit') : t('admin.upstreams.add')"
      width="normal"
      @close="closeForm"
    >
      <form class="space-y-5" @submit.prevent="submitForm">
        <div class="grid gap-4 sm:grid-cols-2">
          <div>
            <label class="input-label">{{ t('admin.upstreams.name') }}</label>
            <input v-model.trim="form.name" class="input" required />
          </div>
          <div class="grid grid-cols-[1fr_100px] gap-3">
            <div>
              <label class="input-label">{{ t('admin.upstreams.kind') }}</label>
            <div class="relative">
              <button
                ref="protocolTrigger"
                type="button"
                class="input flex min-h-11 w-full items-center justify-between bg-gray-50/70 text-left transition-colors focus:border-primary-500 focus:bg-white focus:ring-2 focus:ring-primary-500/20 dark:bg-dark-900/40 dark:focus:bg-dark-800"
                role="combobox"
                aria-haspopup="listbox"
                :aria-expanded="protocolOpen"
                @click="toggleProtocolMenu"
              >
                <span>{{ kindLabel(form.kind) }}</span>
                <Icon name="chevronDown" size="sm" class="text-gray-400 transition-transform" :class="protocolOpen ? 'rotate-180' : ''" />
              </button>
              <Teleport to="body">
                <div v-if="protocolOpen" ref="protocolMenu" :style="protocolMenuStyle" class="fixed z-[1000] overflow-hidden rounded-lg border border-gray-200 bg-white p-1.5 shadow-xl dark:border-dark-600 dark:bg-dark-800" role="listbox">
                <button
                  v-for="option in protocolOptions"
                  :key="option.value"
                  type="button"
                  role="option"
                  :aria-selected="form.kind === option.value"
                  class="flex min-h-10 w-full items-center justify-between rounded-md px-3 py-2 text-left text-sm text-gray-700 transition-colors hover:bg-primary-50 hover:text-primary-700 dark:text-gray-200 dark:hover:bg-primary-900/20 dark:hover:text-primary-300"
                  :class="form.kind === option.value ? 'bg-primary-50 font-medium text-primary-700 dark:bg-primary-900/20 dark:text-primary-300' : ''"
                  @click="selectProtocol(option.value)"
                >
                  {{ option.label }}
                  <Icon v-if="form.kind === option.value" name="check" size="sm" />
                </button>
                </div>
              </Teleport>
            </div>
            </div>
            <div>
              <label class="input-label">{{ t('admin.upstreams.sortCode') }}</label>
              <input v-model.number="form.sort_code" class="input" type="number" min="0" step="1" />
            </div>
          </div>
        </div>

        <div>
          <label class="input-label">{{ t('admin.upstreams.baseUrl') }}</label>
          <input
            v-model.trim="form.base_url"
            class="input"
            type="url"
            placeholder="https://example.com"
            required
          />
        </div>

        <div>
          <label class="input-label">
            {{ t('admin.upstreams.token') }}
            <span class="ml-1 font-normal text-gray-400">{{ t('common.optional') }}</span>
          </label>
          <input
            v-model="form.token"
            class="input"
            type="password"
            autocomplete="new-password"
          />
        </div>

        <div v-if="form.kind === 'sub2api'">
          <label class="input-label">
            {{ t('admin.upstreams.refreshToken') }}
            <span class="ml-1 font-normal text-gray-400">{{ t('common.optional') }}</span>
          </label>
          <input
            v-model="form.refresh_token"
            class="input"
            type="password"
            autocomplete="new-password"
          />
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.upstreams.refreshTokenHint') }}</p>
        </div>

        <div v-if="form.kind === 'newapi'">
          <label class="input-label">{{ t('admin.upstreams.remoteUserId') }}</label>
          <input
            v-model.trim="form.remote_user_id"
            class="input"
            inputmode="numeric"
          />
        </div>

        <div class="grid gap-4 sm:grid-cols-2">
          <div>
            <label class="input-label">{{ form.kind === 'newapi' ? t('admin.upstreams.loginAccount') : t('admin.upstreams.identifier') }}</label>
            <input
              v-model.trim="form.login_identifier"
              class="input"
              :type="form.kind === 'sub2api' ? 'email' : 'text'"
              autocomplete="username"
            />
          </div>
          <div>
            <label class="input-label">{{ t('admin.upstreams.password') }}</label>
            <input
              v-model="form.password"
              class="input"
              type="password"
              autocomplete="new-password"
            />
          </div>
        </div>

        <div>
          <label class="input-label">{{ t('admin.upstreams.notes') }}</label>
          <textarea v-model.trim="form.notes" class="input min-h-20 resize-y" />
        </div>

        <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
          <input v-model="form.enabled" type="checkbox" />
          {{ t('admin.upstreams.enabled') }}
        </label>
      </form>

      <template #footer>
        <div class="flex justify-end gap-3">
          <button
            type="button"
            class="btn btn-secondary"
            @click="closeForm"
          >
            {{ t('admin.upstreams.cancel') }}
          </button>
          <button
            type="button"
            class="btn btn-primary"
            :disabled="saving"
            @click="submitForm"
          >
            {{ saving ? t('common.saving') : t('admin.upstreams.save') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- Create Key Dialog -->
    <BaseDialog
      :show="showKeyDialog"
      :title="t('admin.upstreams.createKey')"
      width="normal"
      @close="closeKeyDialog"
    >
      <div class="space-y-5">
        <div>
          <label class="input-label">{{ t('admin.upstreams.keyName') }}</label>
          <input v-model.trim="keyForm.name" class="input" />
        </div>
        <div>
          <label class="input-label">{{ t('admin.upstreams.group') }}</label>
          <div class="relative">
            <button
              ref="groupTrigger"
              type="button"
              class="input flex min-h-11 w-full items-center justify-between bg-gray-50/70 text-left transition-colors focus:border-primary-500 focus:bg-white focus:ring-2 focus:ring-primary-500/20 dark:bg-dark-900/40 dark:focus:bg-dark-800"
              role="combobox"
              aria-haspopup="listbox"
              :aria-expanded="groupOpen"
              @click="toggleGroupMenu"
            >
              <span :class="keyForm.group ? 'text-gray-800 dark:text-gray-100' : 'text-gray-400'">{{ selectedGroupLabel }}</span>
              <Icon name="chevronDown" size="sm" class="text-gray-400 transition-transform" :class="groupOpen ? 'rotate-180' : ''" />
            </button>
            <Teleport to="body">
              <div v-if="groupOpen" ref="groupMenu" :style="groupMenuStyle" class="fixed z-[1000] max-h-60 overflow-y-auto rounded-lg border border-gray-200 bg-white p-1.5 shadow-xl dark:border-dark-600 dark:bg-dark-800" role="listbox">
              <div v-if="!selectedGroups.length" class="px-3 py-3 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.upstreams.noGroups') }}</div>
              <button
                v-for="group in selectedGroups"
                :key="group.name"
                type="button"
                role="option"
                :aria-selected="keyForm.group === group.name"
                class="flex min-h-10 w-full items-center justify-between gap-3 rounded-md px-3 py-2 text-left text-sm text-gray-700 transition-colors hover:bg-primary-50 hover:text-primary-700 dark:text-gray-200 dark:hover:bg-primary-900/20 dark:hover:text-primary-300"
                :class="keyForm.group === group.name ? 'bg-primary-50 font-medium text-primary-700 dark:bg-primary-900/20 dark:text-primary-300' : ''"
                @click="selectGroup(group.name)"
              >
                <span class="min-w-0 truncate">{{ group.name }}</span>
                <span class="shrink-0 text-xs text-gray-500 dark:text-gray-400">{{ ratioLabel(group.ratio) }}</span>
              </button>
              </div>
            </Teleport>
          </div>
        </div>
      </div>

      <template #footer>
        <div class="flex justify-end gap-3">
          <button
            type="button"
            class="btn btn-secondary"
            @click="closeKeyDialog"
          >
            {{ t('admin.upstreams.cancel') }}
          </button>
          <button
            type="button"
            class="btn btn-primary"
            :disabled="saving || !keyForm.name || !keyForm.group"
            @click="submitKey"
          >
            {{ saving ? t('common.creating') : t('admin.upstreams.createKey') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- Sync Dialog -->
    <BaseDialog
      :show="showSyncDialog"
      :title="t('admin.upstreams.sync')"
      width="normal"
      @close="showSyncDialog = false"
    >
      <div class="space-y-4">
        <p class="text-sm text-gray-500">
          {{ selectedResource?.name }}
        </p>

        <div v-if="localGroups.length" class="grid max-h-72 gap-2 overflow-y-auto sm:grid-cols-2">
          <label
            v-for="group in localGroups"
            :key="group.id"
            class="flex cursor-pointer items-center gap-2 rounded-lg border border-gray-200 px-3 py-2 text-sm transition-colors hover:border-gray-300 dark:border-dark-700 dark:hover:border-dark-600"
          >
            <input
              v-model="syncGroupIds"
              type="checkbox"
              :value="group.id"
            />
            {{ group.name }}
          </label>
        </div>
        <p v-else class="py-4 text-center text-sm text-gray-500">
          {{ t('common.noGroupsAvailable') }}
        </p>
      </div>

      <template #footer>
        <div class="flex justify-end gap-3">
          <button
            type="button"
            class="btn btn-secondary"
            @click="showSyncDialog = false"
          >
            {{ t('admin.upstreams.cancel') }}
          </button>
          <button
            type="button"
            class="btn btn-primary"
            :disabled="saving || !syncGroupIds.length"
            @click="submitSync"
          >
            {{ saving ? t('common.submitting') : t('admin.upstreams.syncSubmit') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- Chat Dialog -->
    <BaseDialog
      :show="showChatDialog"
      :title="`${t('admin.upstreams.chat')} · ${selectedResource?.name || ''}`"
      width="wide"
      @close="closeChat"
    >
      <div class="space-y-4">
        <!-- Controls -->
        <div class="grid gap-3 sm:grid-cols-[minmax(0,1fr)_minmax(0,2fr)]">
          <div>
            <label class="input-label">{{ t('admin.upstreams.models') }}</label>
            <select v-model="chat.model.value" class="input">
              <option
                v-for="m in selectedResource?.models_snapshot || []"
                :key="m"
                :value="m"
              >
                {{ m }}
              </option>
            </select>
          </div>
          <div>
            <label class="input-label">{{ t('admin.upstreams.prompt') }}</label>
            <div class="flex gap-2">
              <input
                v-model.trim="chat.prompt.value"
                class="input"
                @keydown.enter.prevent="handleSendChat"
              />
              <button
                type="button"
                class="btn btn-primary shrink-0"
                :disabled="chat.loading.value || !chat.prompt.value || !chat.model.value"
                @click="handleSendChat"
              >
                <Icon name="arrowRight" size="sm" class="mr-1" />
                {{ t('admin.upstreams.send') }}
              </button>
            </div>
          </div>
        </div>

        <!-- Messages -->
        <div
          class="max-h-96 min-h-[200px] space-y-3 overflow-y-auto rounded-lg border border-gray-200 bg-gray-50 p-4 text-sm dark:border-dark-700 dark:bg-dark-800"
        >
          <div
            v-for="(message, index) in chat.messages.value"
            :key="`${message.role}-${index}`"
            :class="message.role === 'user' ? 'ml-12 flex flex-col items-end' : 'mr-12'"
          >
            <span class="mb-1 flex items-center gap-1 text-xs text-gray-500">
              <Icon :name="message.role === 'user' ? 'user' : 'sparkles'" size="xs" />
              {{ message.role === 'user' ? t('common.user') : t('common.assistant') }}
            </span>
            <div
              :class="[
                'whitespace-pre-wrap rounded-lg px-3 py-2 text-left text-sm shadow-sm',
                message.role === 'user'
                  ? 'bg-blue-50 text-blue-900 dark:bg-blue-900/20 dark:text-blue-100'
                  : 'bg-white text-gray-800 dark:bg-dark-700 dark:text-gray-200'
              ]"
            >
              {{ message.content || '...' }}
            </div>
          </div>
          <div
            v-if="!chat.messages.value.length"
            class="flex flex-col items-center justify-center py-12 text-gray-400"
          >
            <Icon name="chatBubble" size="xl" class="mb-2 text-gray-300 dark:text-dark-500" />
            <p class="text-sm">{{ t('common.noData') }}</p>
          </div>
        </div>
      </div>
    </BaseDialog>

    <!-- Delete Confirmation -->
    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('admin.upstreams.delete')"
      :message="t('admin.upstreams.deleteConfirm')"
      :confirm-text="t('admin.upstreams.delete')"
      :cancel-text="t('admin.upstreams.cancel')"
      :danger="true"
      @confirm="executeDelete"
      @cancel="showDeleteDialog = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { adminAPI } from '@/api'
import { useAppStore } from '@/stores/app'
import { useUpstreamChat } from '@/composables/useUpstreamChat'
import type {
  AdminGroup,
  Upstream,
  UpstreamGroupItem,
  UpstreamInput,
  UpstreamResource
} from '@/types'

const { t } = useI18n()
const appStore = useAppStore()
const chat = useUpstreamChat()

// ── Core State ──────────────────────────────────────────────

const upstreams = ref<Upstream[]>([])
const groupsByUpstream = ref<Record<number, UpstreamGroupItem[]>>({})
const resourcesByUpstream = ref<Record<number, UpstreamResource[]>>({})
const expanded = ref(new Set<number>())
const totalUpstreams = ref(0)
const currentPage = ref(1)
const pageSize = ref(20)
const totalPages = ref(1)
const searchQuery = ref('')
let searchTimer: ReturnType<typeof setTimeout> | null = null
const loading = ref(false)
let upstreamLoadRequest = 0
const saving = ref(false)
const busyId = ref<number | null>(null)
const balanceBusyId = ref<number | null>(null)
const groupLoadingIds = ref(new Set<number>())
const resourceLoadingIds = ref(new Set<number>())
const balanceSettings = reactive({ enabled: false, threshold: 0, emails: [] as string[] })
const balanceSettingsSaving = ref(false)
const refreshAllBusy = ref(false)
let balanceTimer: ReturnType<typeof setInterval> | null = null

const activeUpstreamCount = computed(() => upstreams.value.filter((upstream) => upstream.enabled).length)
const resourceCount = computed(() => upstreams.value.reduce((sum, u) => sum + (u.resource_count ?? 0), 0))
const protocolOptions = computed(() => [
  { value: 'newapi', label: t('admin.upstreams.newapi') },
  { value: 'sub2api', label: t('admin.upstreams.sub2api') }
])
const selectedGroupLabel = computed(() => {
  if (!keyForm.group) return t('common.selectOption')
  const selected = selectedGroups.value.find((group) => group.name === keyForm.group)
  return selected ? `${selected.name} · ${ratioLabel(selected.ratio)}` : keyForm.group
})

// ── Form State ──────────────────────────────────────────────

const showForm = ref(false)
const editingId = ref<number | null>(null)
const protocolOpen = ref(false)
const protocolTrigger = ref<HTMLElement | null>(null)
const protocolMenu = ref<HTMLElement | null>(null)
const protocolMenuStyle = ref<Record<string, string>>({})
const form = reactive<UpstreamInput>({
  name: '',
  sort_code: 0,
  kind: 'newapi',
  base_url: '',
  token: '',
  refresh_token: '',
  login_identifier: '',
  remote_user_id: '',
  password: '',
  notes: '',
  enabled: true
})

// ── Key Dialog State ────────────────────────────────────────

const showKeyDialog = ref(false)
const selectedUpstream = ref<Upstream | null>(null)
const selectedGroups = ref<UpstreamGroupItem[]>([])
const keyForm = reactive({ name: '', group: '' })
const groupOpen = ref(false)
const groupTrigger = ref<HTMLElement | null>(null)
const groupMenu = ref<HTMLElement | null>(null)
const groupMenuStyle = ref<Record<string, string>>({})

// ── Sync Dialog State ───────────────────────────────────────

const showSyncDialog = ref(false)
const selectedResource = ref<UpstreamResource | null>(null)
const localGroups = ref<AdminGroup[]>([])
const syncGroupIds = ref<number[]>([])

// ── Chat Dialog State ───────────────────────────────────────

const showChatDialog = ref(false)

// ── Delete Dialog State ─────────────────────────────────────

const showDeleteDialog = ref(false)
const deletingUpstream = ref<Upstream | null>(null)

// ── Helpers ─────────────────────────────────────────────────

function resetForm() {
  Object.assign(form, {
    name: '',
    sort_code: 0,
    kind: 'newapi',
    base_url: '',
    token: '',
    refresh_token: '',
    login_identifier: '',
    remote_user_id: '',
    password: '',
    notes: '',
    enabled: true
  })
}

function kindLabel(kind: string) {
  return kind === 'sub2api'
    ? t('admin.upstreams.sub2api')
    : t('admin.upstreams.newapi')
}

function kindTileClass(kind: string) {
  return kind === 'sub2api'
    ? 'bg-gradient-to-br from-violet-500 to-purple-600 shadow-violet-500/30'
    : 'bg-gradient-to-br from-primary-500 to-blue-600 shadow-primary-500/30'
}

function kindBadgeClass(kind: string) {
  return kind === 'sub2api' ? 'badge-purple' : 'badge-primary'
}

function selectProtocol(kind: string) {
  form.kind = kind
  protocolOpen.value = false
}

function selectGroup(group: string) {
  keyForm.group = group
  groupOpen.value = false
}

function dropdownStyle(trigger: HTMLElement | null, maxHeight: number): Record<string, string> {
  if (!trigger) return {}
  const rect = trigger.getBoundingClientRect()
  const gap = 6
  const viewportPadding = 12
  const availableBelow = window.innerHeight - rect.bottom - viewportPadding - gap
  const availableAbove = rect.top - viewportPadding - gap
  const height = Math.min(maxHeight, Math.max(80, Math.max(availableBelow, availableAbove)))
  const openAbove = availableBelow < height && availableAbove > availableBelow
  const top = openAbove ? rect.top - gap - height : rect.bottom + gap
  const left = Math.min(Math.max(viewportPadding, rect.left), window.innerWidth - rect.width - viewportPadding)
  return {
    top: `${Math.max(viewportPadding, top)}px`,
    left: `${left}px`,
    width: `${rect.width}px`,
    maxHeight: `${height}px`
  }
}

async function toggleProtocolMenu() {
  groupOpen.value = false
  protocolOpen.value = !protocolOpen.value
  if (protocolOpen.value) {
    await nextTick()
    protocolMenuStyle.value = dropdownStyle(protocolTrigger.value, 120)
  }
}

async function toggleGroupMenu() {
  protocolOpen.value = false
  groupOpen.value = !groupOpen.value
  if (groupOpen.value) {
    await nextTick()
    groupMenuStyle.value = dropdownStyle(groupTrigger.value, 240)
  }
}

function repositionOpenMenus() {
  if (protocolOpen.value) protocolMenuStyle.value = dropdownStyle(protocolTrigger.value, 120)
  if (groupOpen.value) groupMenuStyle.value = dropdownStyle(groupTrigger.value, 240)
}

function ratioLabel(ratio: number | null | undefined) {
  return ratio === null || ratio === undefined
    ? t('admin.upstreams.autoRatio')
    : String(ratio)
}

function numberLabel(value: unknown) {
  if (typeof value === 'number') return value.toLocaleString()
  if (typeof value === 'string' && value.trim()) return value
  return '-'
}

function remainingValue(upstream: Upstream): number | undefined {
  const snapshot = upstream.balance_snapshot
  if (typeof snapshot?.remaining === 'number') return snapshot.remaining
  if (typeof snapshot?.quota === 'number') {
    if (typeof snapshot.used_quota === 'number' && snapshot.used_quota > 0) {
      return Math.max(snapshot.quota - snapshot.used_quota, 0)
    }
    return snapshot.quota
  }
  return undefined
}

function remainingLabel(upstream: Upstream) {
  const value = remainingValue(upstream)
  return value === undefined ? '-' : value.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

function currencyLabel(upstream: Upstream) {
  return upstream.balance_snapshot?.currency || 'USD'
}

function freshnessLabel(upstream: Upstream) {
  const value = upstream.balance_snapshot?.fetched_at || upstream.last_checked_at || undefined
  if (!value) return '-'
  const timestamp = new Date(value).getTime()
  if (Number.isNaN(timestamp)) return value
  const minutes = Math.floor(Math.max(Date.now() - timestamp, 0) / 60000)
  if (minutes < 1) return t('admin.upstreams.justNow')
  if (minutes < 60) return t('admin.upstreams.minutesAgo', { count: minutes })
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return t('admin.upstreams.hoursAgo', { count: hours })
  return dateLabel(value)
}

function dateLabel(value: string | undefined) {
  if (!value) return '-'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}

function errorMessage(error: unknown): string {
  if (error && typeof error === 'object' && 'message' in error) {
    const err = error as Record<string, unknown>
    const msg = String(err.message)
    // Surface upstream login error reason for clearer feedback
    if (err.reason === 'LOGIN_FAILED') {
      return `${t('admin.upstreams.loginFailed')}: ${msg}`
    }
    return msg
  }
  return t('admin.upstreams.requestFailed')
}

// ── Data Loading ────────────────────────────────────────────

async function loadUpstreams() {
  const requestId = ++upstreamLoadRequest
  loading.value = true
  try {
    const result = await adminAPI.upstreams.listPaginated({
      page: currentPage.value,
      page_size: pageSize.value,
      search: searchQuery.value || undefined
    })
    if (requestId !== upstreamLoadRequest) return
    upstreams.value = result.items
    totalUpstreams.value = result.total
    totalPages.value = result.pages
    pruneDetailCaches()
  } catch (error) {
    if (requestId === upstreamLoadRequest) appStore.showError(errorMessage(error))
  } finally {
    if (requestId === upstreamLoadRequest) loading.value = false
  }
}

async function refreshUpstreams() {
  if (loading.value) return
  await loadUpstreams()
}

// 仅保留当前页上游的展开状态与详情缓存，防止翻页累积导致内存无限增长
function pruneDetailCaches() {
  const visibleIds = new Set(upstreams.value.map((item) => item.id))

  const nextExpanded = new Set<number>()
  for (const id of expanded.value) {
    if (visibleIds.has(id)) nextExpanded.add(id)
  }
  expanded.value = nextExpanded

  const nextGroups: Record<number, UpstreamGroupItem[]> = {}
  for (const [id, groups] of Object.entries(groupsByUpstream.value)) {
    const numericId = Number(id)
    if (visibleIds.has(numericId)) nextGroups[numericId] = groups
  }
  groupsByUpstream.value = nextGroups

  const nextResources: Record<number, UpstreamResource[]> = {}
  for (const [id, resources] of Object.entries(resourcesByUpstream.value)) {
    const numericId = Number(id)
    if (visibleIds.has(numericId)) nextResources[numericId] = resources
  }
  resourcesByUpstream.value = nextResources

  const nextGroupLoading = new Set<number>()
  for (const id of groupLoadingIds.value) {
    if (visibleIds.has(id)) nextGroupLoading.add(id)
  }
  groupLoadingIds.value = nextGroupLoading

  const nextLoading = new Set<number>()
  for (const id of resourceLoadingIds.value) {
    if (visibleIds.has(id)) nextLoading.add(id)
  }
  resourceLoadingIds.value = nextLoading
}

async function loadBalanceSettings() {
  try {
    const result = await adminAPI.upstreams.getBalanceNotifySettings()
    balanceSettings.enabled = result.enabled
    balanceSettings.threshold = result.threshold
    balanceSettings.emails = [...(result.emails || [])]
  } catch (error) { appStore.showError(errorMessage(error)) }
}

const newNotifyEmail = ref('')
function addNotifyEmailFromInput() {
  const email = newNotifyEmail.value.trim()
  if (!email) return
  balanceSettings.emails.push(email)
  newNotifyEmail.value = ''
}
function removeNotifyEmail(index: number) { balanceSettings.emails.splice(index, 1) }

async function saveBalanceSettings() {
  balanceSettings.threshold = Math.min(100, Math.max(0, Number(balanceSettings.threshold) || 0))
  balanceSettings.emails = balanceSettings.emails.map(email => email.trim()).filter(Boolean)
  balanceSettingsSaving.value = true
  try {
    const result = await adminAPI.upstreams.updateBalanceNotifySettings({ ...balanceSettings, emails: [...balanceSettings.emails] })
    balanceSettings.emails = [...result.emails]
    appStore.showSuccess(t('admin.upstreams.balanceSettingsSaved'))
    resetBalanceTimer()
  } catch (error) { appStore.showError(errorMessage(error)) } finally { balanceSettingsSaving.value = false }
}

async function refreshAllBalances() {
  refreshAllBusy.value = true
  try {
    const updated = await adminAPI.upstreams.refreshAllBalances()
    for (const item of updated) { const index = upstreams.value.findIndex(current => current.id === item.id); if (index >= 0) upstreams.value[index] = item }
    appStore.showSuccess(t('admin.upstreams.allBalancesUpdated'))
  } catch (error) { appStore.showError(errorMessage(error)) } finally { refreshAllBusy.value = false }
}

function resetBalanceTimer() {
  if (balanceTimer) { clearInterval(balanceTimer); balanceTimer = null }
  if (balanceSettings.enabled) balanceTimer = setInterval(() => { void refreshAllBalances() }, 30000)
}

function goToPage(page: number) {
  if (page < 1 || page > totalPages.value || page === currentPage.value) return
  currentPage.value = page
  loadUpstreams()
}

function onSearchInput() {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    currentPage.value = 1
    loadUpstreams()
  }, 300)
}

async function loadGroups(upstream: Upstream, refresh = false) {
  const ids = new Set(groupLoadingIds.value)
  ids.add(upstream.id)
  groupLoadingIds.value = ids
  try {
    const groups = await adminAPI.upstreams.groups(upstream.id, refresh)
    groupsByUpstream.value = {
      ...groupsByUpstream.value,
      [upstream.id]: groups
    }
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    const done = new Set(groupLoadingIds.value)
    done.delete(upstream.id)
    groupLoadingIds.value = done
  }
}

async function loadResources(upstream: Upstream) {
  const ids = new Set(resourceLoadingIds.value)
  ids.add(upstream.id)
  resourceLoadingIds.value = ids
  try {
    const resources = await adminAPI.upstreams.resources(upstream.id)
    resourcesByUpstream.value = {
      ...resourcesByUpstream.value,
      [upstream.id]: resources
    }
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    const done = new Set(resourceLoadingIds.value)
    done.delete(upstream.id)
    resourceLoadingIds.value = done
  }
}

async function loadDetails(upstream: Upstream, refresh = false) {
  await Promise.all([loadGroups(upstream, refresh), loadResources(upstream)])
}


async function toggleExpanded(upstream: Upstream) {
  const next = new Set(expanded.value)
  if (next.has(upstream.id)) {
    next.delete(upstream.id)
    expanded.value = next
    return
  } else {
    next.add(upstream.id)
    expanded.value = next
    // Load groups and resources in parallel on first expand
    const promises: Promise<void>[] = []
    if (!groupsByUpstream.value[upstream.id]) {
      promises.push(loadGroups(upstream))
    }
    if (!resourcesByUpstream.value[upstream.id]) {
      promises.push(loadResources(upstream))
    }
    if (promises.length) {
      await Promise.all(promises)
    }
  }
}

// ── Create / Edit ───────────────────────────────────────────

function openCreate() {
  resetForm()
  protocolOpen.value = false
  editingId.value = null
  showForm.value = true
}

function closeForm() {
  protocolOpen.value = false
  showForm.value = false
}

function openEdit(upstream: Upstream) {
  Object.assign(form, {
    name: upstream.name,
    sort_code: upstream.sort_code ?? 0,
    kind: upstream.kind,
    base_url: upstream.base_url,
    token: '',
    refresh_token: '',
    login_identifier: upstream.login_identifier || '',
    remote_user_id: upstream.remote_user_id || '',
    password: '',
    notes: upstream.notes || '',
    enabled: upstream.enabled
  })
  protocolOpen.value = false
  editingId.value = upstream.id
  showForm.value = true
}

async function submitForm() {
  if (!form.name || !form.base_url) {
    appStore.showError(t('admin.upstreams.missingForm'))
    return
  }
  if (!editingId.value && !form.token && !form.refresh_token && (!form.login_identifier || !form.password)) {
    appStore.showError(t('admin.upstreams.missingLogin'))
    return
  }
  if (!editingId.value && form.kind === 'newapi' && form.token && !form.remote_user_id) {
    appStore.showError(t('admin.upstreams.missingRemoteUserId'))
    return
  }

  saving.value = true
  try {
    let result: Upstream
    if (editingId.value) {
      result = await adminAPI.upstreams.update(editingId.value, form)
    } else {
      result = await adminAPI.upstreams.create(form)
    }
    showForm.value = false

    // Show warning if connection test failed during save
    if (result.last_error) {
      appStore.showWarning(
        `${t('admin.upstreams.saved')} — ${result.last_error}`
      )
    } else {
      appStore.showSuccess(t('admin.upstreams.saved'))
    }

    await loadUpstreams()
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    saving.value = false
  }
}

// ── Test Connection ─────────────────────────────────────────

async function testConnection(upstream: Upstream) {
  busyId.value = upstream.id
  try {
    const updated = await adminAPI.upstreams.test(upstream.id)
    const index = upstreams.value.findIndex(item => item.id === upstream.id)
    if (index >= 0) {
      upstreams.value[index] = updated
    }
    appStore.showSuccess(t('admin.upstreams.tested'))
    if (expanded.value.has(upstream.id)) {
      await loadDetails(updated)
    }
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    busyId.value = null
  }
}

async function refreshBalance(upstream: Upstream) {
  balanceBusyId.value = upstream.id
  try {
    const updated = await adminAPI.upstreams.refreshBalance(upstream.id)
    const index = upstreams.value.findIndex((item) => item.id === upstream.id)
    if (index >= 0) upstreams.value[index] = updated
    appStore.showSuccess(t('admin.upstreams.balanceUpdated'))
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    balanceBusyId.value = null
  }
}

// ── Delete ──────────────────────────────────────────────────

function confirmDeleteUpstream(upstream: Upstream) {
  deletingUpstream.value = upstream
  showDeleteDialog.value = true
}

async function executeDelete() {
  if (!deletingUpstream.value) return
  try {
    await adminAPI.upstreams.remove(deletingUpstream.value.id)
    appStore.showSuccess(t('admin.upstreams.deleted'))
    showDeleteDialog.value = false
    deletingUpstream.value = null
    await loadUpstreams()
  } catch (error) {
    appStore.showError(errorMessage(error))
  }
}

// ── Key Management ──────────────────────────────────────────

function openKey(upstream: Upstream) {
  selectedUpstream.value = upstream
  selectedGroups.value = groupsByUpstream.value[upstream.id] || []
  Object.assign(keyForm, { name: `${upstream.name}-key`, group: '' })
  groupOpen.value = false
  showKeyDialog.value = true
}

function closeKeyDialog() {
  groupOpen.value = false
  showKeyDialog.value = false
}

async function submitKey() {
  if (!selectedUpstream.value) return
  saving.value = true
  try {
    await adminAPI.upstreams.createKey(selectedUpstream.value.id, keyForm)
    showKeyDialog.value = false
    appStore.showSuccess(t('admin.upstreams.keyCreated'))
    await loadDetails(selectedUpstream.value)
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    saving.value = false
  }
}

// ── Model Refresh ───────────────────────────────────────────

async function refreshResourceModels(resource: UpstreamResource) {
  try {
    const models = await adminAPI.upstreams.refreshModels(resource.id)
    resource.models_snapshot = models
    appStore.showSuccess(t('admin.upstreams.modelsUpdated'))
  } catch (error) {
    appStore.showError(errorMessage(error))
  }
}

// ── Sync to Account ─────────────────────────────────────────

async function openSync(resource: UpstreamResource) {
  selectedResource.value = resource
  syncGroupIds.value = []
  try {
    localGroups.value = await adminAPI.groups.getAll()
  } catch (error) {
    appStore.showError(errorMessage(error))
    return
  }
  showSyncDialog.value = true
}

async function submitSync() {
  if (!selectedResource.value || !syncGroupIds.value.length) {
    appStore.showError(t('admin.upstreams.missingGroup'))
    return
  }
  saving.value = true
  try {
    const result = await adminAPI.upstreams.syncToAccount(
      selectedResource.value.id,
      syncGroupIds.value
    )
    showSyncDialog.value = false
    appStore.showSuccess(
      `${t('admin.upstreams.syncDone')}: ${result.created + result.updated}`
    )
    const upstream = upstreams.value.find(
      item => item.id === selectedResource.value?.upstream_id
    )
    if (upstream) await loadDetails(upstream)
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    saving.value = false
  }
}

// ── Chat ────────────────────────────────────────────────────

function openChat(resource: UpstreamResource) {
  selectedResource.value = resource
  chat.initForResource(resource)
  showChatDialog.value = true
}

function closeChat() {
  showChatDialog.value = false
  chat.reset()
}

async function handleSendChat() {
  if (!selectedResource.value) return
  await chat.send(selectedResource.value)
}

// ── Init ────────────────────────────────────────────────────

onMounted(() => {
  loadUpstreams()
  loadBalanceSettings()
  window.addEventListener('resize', repositionOpenMenus)
  window.addEventListener('scroll', repositionOpenMenus, true)
})

watch(() => balanceSettings.enabled, resetBalanceTimer)

onBeforeUnmount(() => {
  window.removeEventListener('resize', repositionOpenMenus)
  window.removeEventListener('scroll', repositionOpenMenus, true)
  if (balanceTimer) clearInterval(balanceTimer)
  if (searchTimer) clearTimeout(searchTimer)
  chat.reset()
})
</script>
