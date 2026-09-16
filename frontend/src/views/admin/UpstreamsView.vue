<template>
  <AppLayout>
    <div class="mx-auto max-w-7xl space-y-6 p-4 sm:p-6 lg:space-y-7">
      <!-- Page Header -->
      <div
        class="flex flex-col gap-5 border-b border-gray-200 pb-6 sm:flex-row sm:items-end sm:justify-between dark:border-dark-700"
      >
        <div class="min-w-0">
          <div class="mb-2 flex items-center gap-2 text-xs font-medium uppercase text-primary-600 dark:text-primary-400">
            <Icon name="cloud" size="sm" />
            {{ t('admin.upstreams.eyebrow') }}
          </div>
          <h1 class="text-2xl font-semibold leading-tight text-gray-900 dark:text-white sm:text-3xl">
            {{ t('admin.upstreams.title') }}
          </h1>
          <p class="mt-2 max-w-2xl text-sm leading-6 text-gray-500 dark:text-gray-400">
            {{ t('admin.upstreams.description') }}
          </p>
        </div>
        <button type="button" class="btn btn-primary shrink-0 self-start sm:self-auto" @click="openCreate">
          <Icon name="plus" size="sm" class="mr-1.5" />
          {{ t('admin.upstreams.add') }}
        </button>
      </div>

      <div v-if="!loading && upstreams.length" class="grid gap-3 sm:grid-cols-3">
        <div class="rounded-lg border border-gray-200 bg-white px-4 py-3 shadow-sm dark:border-dark-700 dark:bg-dark-800">
          <span class="block text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.upstreams.total') }}</span>
          <strong class="mt-1 block text-xl font-semibold text-gray-900 dark:text-white">{{ upstreams.length }}</strong>
        </div>
        <div class="rounded-lg border border-gray-200 bg-white px-4 py-3 shadow-sm dark:border-dark-700 dark:bg-dark-800">
          <span class="block text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.upstreams.active') }}</span>
          <strong class="mt-1 block text-xl font-semibold text-green-700 dark:text-green-400">{{ activeUpstreamCount }}</strong>
        </div>
        <div class="rounded-lg border border-gray-200 bg-white px-4 py-3 shadow-sm dark:border-dark-700 dark:bg-dark-800">
          <span class="block text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.upstreams.keyCount') }}</span>
          <strong class="mt-1 block text-xl font-semibold text-primary-700 dark:text-primary-300">{{ resourceCount }}</strong>
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
        <Icon name="cloud" size="xl" class="mx-auto mb-3 text-gray-300 dark:text-dark-500" />
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
          class="card overflow-hidden p-0 shadow-sm transition-shadow hover:shadow-md"
        >
          <!-- Card Header -->
          <div class="flex flex-col gap-4 p-5 xl:flex-row xl:items-center xl:justify-between sm:p-6">
            <button
              type="button"
              class="flex min-w-0 items-start gap-3 text-left"
              :aria-expanded="expanded.has(upstream.id)"
              :aria-label="`${expanded.has(upstream.id) ? t('admin.upstreams.collapse') : t('admin.upstreams.expand')} ${upstream.name}`"
              @click="toggleExpanded(upstream)"
            >
              <Icon
                :name="expanded.has(upstream.id) ? 'chevronDown' : 'chevronRight'"
                size="md"
                class="mt-0.5 shrink-0 text-gray-400"
              />
              <span class="min-w-0">
                <span class="flex flex-wrap items-center gap-2">
                  <span class="truncate font-medium text-gray-900 dark:text-white">
                    {{ upstream.name }}
                  </span>
                  <span class="badge badge-gray">{{ kindLabel(upstream.kind) }}</span>
                  <span
                    :class="[
                      'badge',
                      upstream.enabled ? 'badge-green' : 'badge-gray'
                    ]"
                  >
                    {{ upstream.enabled ? t('common.enabled') : t('common.disabled') }}
                  </span>
                </span>
                <span class="mt-1 block truncate text-xs text-gray-500 dark:text-gray-400">
                  {{ upstream.base_url }}
                </span>
              </span>
            </button>

            <!-- Card Actions -->
            <div class="flex flex-wrap items-center gap-2 xl:justify-end">
              <div class="flex min-w-[210px] items-center gap-2 rounded-lg bg-gray-50 px-3 py-2 dark:bg-dark-800">
                <span
                  class="inline-flex items-center gap-1 text-xs text-gray-500 dark:text-gray-400"
                  :title="upstream.balance_snapshot?.fetched_at ? dateLabel(upstream.balance_snapshot.fetched_at) : undefined"
                >
                  <Icon name="clock" size="xs" />
                  {{ freshnessLabel(upstream) }}
                </span>
                <button
                  type="button"
                  class="inline-flex min-h-8 min-w-8 items-center justify-center rounded-md text-gray-500 transition-colors hover:bg-white hover:text-primary-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/30 disabled:cursor-not-allowed disabled:opacity-50 dark:hover:bg-dark-700 dark:hover:text-primary-400"
                  :disabled="balanceBusyId === upstream.id"
                  :title="t('admin.upstreams.refreshBalance')"
                  :aria-label="t('admin.upstreams.refreshBalance')"
                  @click.stop="refreshBalance(upstream)"
                >
                  <Icon name="refresh" size="sm" :class="balanceBusyId === upstream.id ? 'animate-spin' : ''" />
                </button>
                <span class="ml-auto inline-flex items-baseline gap-1 whitespace-nowrap">
                  <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.upstreams.remaining') }}</span>
                  <strong class="text-base font-semibold tabular-nums text-emerald-600 dark:text-emerald-400">
                    {{ remainingLabel(upstream) }}
                  </strong>
                  <span class="text-xs text-gray-500 dark:text-gray-400">{{ currencyLabel(upstream) }}</span>
                </span>
              </div>
              <span
                v-if="upstream.last_error"
                class="inline-flex max-w-full items-center gap-1 truncate rounded-md bg-red-50 px-2 py-1 text-xs text-red-700 dark:bg-red-900/20 dark:text-red-300 xl:max-w-xs"
                :title="upstream.last_error"
              >
                <Icon name="exclamationCircle" size="xs" class="shrink-0" />
                {{ upstream.last_error }}
              </span>
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
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                @click="openEdit(upstream)"
              >
                <Icon name="edit" size="sm" class="mr-1" />
                {{ t('common.edit') }}
              </button>
              <button
                type="button"
                class="btn btn-danger btn-sm"
                @click="confirmDeleteUpstream(upstream)"
              >
                <Icon name="trash" size="sm" class="mr-1" />
                {{ t('admin.upstreams.delete') }}
              </button>
              <a
                :href="upstream.base_url"
                target="_blank"
                rel="noopener noreferrer"
                class="btn btn-secondary btn-sm"
              >
                <Icon name="externalLink" size="sm" class="mr-1" />
                {{ t('admin.upstreams.openSite') }}
              </a>
            </div>
          </div>

          <!-- Expanded Details -->
          <div
            v-if="expanded.has(upstream.id)"
            class="border-t border-gray-100 bg-gray-50/60 p-5 dark:border-dark-700 dark:bg-dark-900/30 sm:p-6"
          >
            <div class="grid gap-4 lg:grid-cols-2">
              <!-- Groups Panel -->
              <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
                <div class="mb-3 flex items-center justify-between gap-3">
                  <h2 class="font-medium text-gray-900 dark:text-white">
                    {{ t('admin.upstreams.groups') }}
                  </h2>
                  <button
                    type="button"
                    class="btn btn-ghost btn-sm"
                    :disabled="detailBusy === upstream.id"
                    @click="loadDetails(upstream, true)"
                  >
                    <Icon name="refresh" size="sm" class="mr-1" />
                    {{ t('admin.upstreams.refreshGroups') }}
                  </button>
                </div>

                <!-- Groups Table -->
                <div
                  v-if="groupsByUpstream[upstream.id]?.length"
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
                          <span class="inline-flex items-center rounded-md bg-blue-50 px-2 py-0.5 text-xs font-medium text-blue-700 dark:bg-blue-900/30 dark:text-blue-300">
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
                  class="mt-4 grid grid-cols-3 gap-3 border-t border-gray-100 pt-4 dark:border-dark-700"
                >
                  <div class="rounded-md bg-gray-50 px-3 py-2 dark:bg-dark-800">
                    <span class="block text-xs text-gray-500">{{ t('admin.upstreams.quota') }}</span>
                    <strong class="mt-0.5 block text-sm text-gray-800 dark:text-gray-200">
                      {{ numberLabel(upstream.balance_snapshot?.quota) }}
                    </strong>
                  </div>
                  <div class="rounded-md bg-gray-50 px-3 py-2 dark:bg-dark-800">
                    <span class="block text-xs text-gray-500">{{ t('admin.upstreams.usedQuota') }}</span>
                    <strong class="mt-0.5 block text-sm text-gray-800 dark:text-gray-200">
                      {{ numberLabel(upstream.balance_snapshot?.used_quota) }}
                    </strong>
                  </div>
                  <div class="rounded-md bg-gray-50 px-3 py-2 dark:bg-dark-800">
                    <span class="block text-xs text-gray-500">{{ t('admin.upstreams.requestCount') }}</span>
                    <strong class="mt-0.5 block text-sm text-gray-800 dark:text-gray-200">
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
              <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
                <div class="mb-3 flex items-center justify-between gap-3">
                  <h2 class="font-medium text-gray-900 dark:text-white">
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
                  class="space-y-3"
                >
                  <div
                    v-for="resource in resourcesByUpstream[upstream.id]"
                    :key="resource.id"
                    class="rounded-lg border border-gray-100 p-3 transition-colors hover:border-gray-200 dark:border-dark-700 dark:hover:border-dark-600"
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
                          class="btn btn-ghost btn-sm min-h-10 min-w-10"
                          :title="t('admin.upstreams.refreshModels')"
                          :aria-label="t('admin.upstreams.refreshModels')"
                          @click="refreshResourceModels(resource)"
                        >
                          <Icon name="refresh" size="sm" />
                        </button>
                        <button
                          type="button"
                          class="btn btn-ghost btn-sm min-h-10 min-w-10"
                          :title="t('admin.upstreams.chat')"
                          :aria-label="t('admin.upstreams.chat')"
                          @click="openChat(resource)"
                        >
                          <Icon name="chatBubble" size="sm" />
                        </button>
                        <button
                          type="button"
                          class="btn btn-ghost btn-sm min-h-10 min-w-10"
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
                        class="rounded-md bg-gray-100 px-2 py-0.5 text-xs text-gray-600 dark:bg-dark-700 dark:text-gray-400"
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
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
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
const loading = ref(false)
const saving = ref(false)
const busyId = ref<number | null>(null)
const balanceBusyId = ref<number | null>(null)
const detailBusy = ref<number | null>(null)

const activeUpstreamCount = computed(() => upstreams.value.filter((upstream) => upstream.enabled).length)
const resourceCount = computed(() => Object.values(resourcesByUpstream.value).reduce((total, resources) => total + resources.length, 0))
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
    return String((error as { message?: unknown }).message)
  }
  return t('admin.upstreams.requestFailed')
}

// ── Data Loading ────────────────────────────────────────────

async function load() {
  loading.value = true
  try {
    upstreams.value = await adminAPI.upstreams.list()
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    loading.value = false
  }
}

async function loadDetails(upstream: Upstream, refresh = false) {
  detailBusy.value = upstream.id
  try {
    const [groups, resources] = await Promise.all([
      adminAPI.upstreams.groups(upstream.id, refresh),
      adminAPI.upstreams.resources(upstream.id)
    ])
    groupsByUpstream.value = {
      ...groupsByUpstream.value,
      [upstream.id]: groups
    }
    resourcesByUpstream.value = {
      ...resourcesByUpstream.value,
      [upstream.id]: resources
    }
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    detailBusy.value = null
  }
}

async function toggleExpanded(upstream: Upstream) {
  const next = new Set(expanded.value)
  if (next.has(upstream.id)) {
    next.delete(upstream.id)
  } else {
    next.add(upstream.id)
    await loadDetails(upstream)
  }
  expanded.value = next
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
  if (!editingId.value && !form.token && (!form.login_identifier || !form.password)) {
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

    await load()
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
    await load()
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
  load()
  window.addEventListener('resize', repositionOpenMenus)
  window.addEventListener('scroll', repositionOpenMenus, true)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', repositionOpenMenus)
  window.removeEventListener('scroll', repositionOpenMenus, true)
})
</script>
