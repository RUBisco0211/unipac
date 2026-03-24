<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Search, ArrowUpCircle, Trash2, RefreshCcwIcon } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import PackagesDataTable from '@/components/packages/PackagesDataTable.vue'
import { createPackageColumns } from '@/components/packages/packages-columns'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { finishLoadingBar, startLoadingBar } from '@/composables/useLoadingBar'
import { ensureManagersLoaded, useManagers } from '@/composables/useManagers'
import {
    batchUninstallPackages,
    batchUpgradePackages,
    loadCachedPackages,
    reloadPackages,
    uninstallPackage,
    upgradePackage,
} from '@/lib/api'
import type { ManagerType, Package, PackageTarget } from '@/model/types'
import type { RowSelectionState } from '@tanstack/vue-table'

const searchQuery = ref('')
const filterManager = ref<ManagerType | 'all'>('all')
const loading = ref(false)
const actionLoading = ref<string | null>(null)
const packages = ref<Package[]>([])
const rowSelection = ref<RowSelectionState>({})
const { t, locale } = useI18n()
const { enabledManagers, managerNameMap } = useManagers()

const filteredPackages = computed(() => {
    const query = searchQuery.value.trim().toLowerCase()

    return packages.value.filter(pkg => {
        const matchesManager = filterManager.value === 'all' || filterManager.value === pkg.manager
        const matchesQuery =
            !query ||
            pkg.name.toLowerCase().includes(query) ||
            pkg.fullname?.toLowerCase().includes(query) ||
            pkg.description?.toLowerCase().includes(query)

        return matchesManager && matchesQuery
    })
})

const columns = computed(() => {
    locale.value
    return createPackageColumns({
        mode: 'installed',
        loadingKey: actionLoading.value,
        managerLabels: managerNameMap.value,
        selectable: true,
        onUpgrade: pkg => {
            void handleUpgrade(pkg)
        },
        onUninstall: pkg => {
            void handleUninstall(pkg)
        },
    })
})

const selectedPackages = computed(() => {
    const selectedIds = new Set(
        Object.keys(rowSelection.value).filter(key => rowSelection.value[key])
    )
    return filteredPackages.value.filter(pkg => selectedIds.has(`${pkg.manager}:${pkg.name}`))
})

const hasSelectedPackages = computed(() => selectedPackages.value.length > 0)
const hasSelectedOutdatedPackages = computed(() => selectedPackages.value.some(pkg => pkg.outdated))

async function loadPackages() {
    loading.value = true
    startLoadingBar()

    try {
        await reloadPackages()
        packages.value = await loadCachedPackages()
        rowSelection.value = {}
    } catch (error) {
        toast.error(t('packages.requestFailed'), {
            description: t('packages.loadError', { error: String(error) }),
        })
    } finally {
        loading.value = false
        finishLoadingBar()
    }
}

async function loadPackagesOnMount() {
    // 先加载缓存数据，快速显示
    try {
        packages.value = await loadCachedPackages()
        rowSelection.value = {}
    } catch (error) {
        // 缓存加载失败是正常的（首次启动），忽略错误
    }

    // 异步刷新数据
    void loadPackages()
}

async function handleBulkUpgrade() {
    const targets: PackageTarget[] = selectedPackages.value
        .filter(pkg => pkg.outdated)
        .map(pkg => ({ manager: pkg.manager, name: pkg.name }))

    if (!targets.length) return

    actionLoading.value = 'bulk-upgrade'
    startLoadingBar()

    try {
        const result = await batchUpgradePackages(targets)
        if (!result.success) throw new Error(result.message)
        toast.success(t('packages.operationCompleted'), {
            description: result.message,
        })
        // 操作成功后重新加载数据
        await reloadPackages()
        packages.value = await loadCachedPackages()
        rowSelection.value = {}
    } catch (error) {
        toast.error(t('packages.requestFailed'), {
            description: t('packages.bulkUpdateError', { error: String(error) }),
        })
    } finally {
        actionLoading.value = null
        finishLoadingBar()
    }
}

async function handleBulkUninstall() {
    const targets: PackageTarget[] = selectedPackages.value.map(pkg => ({
        manager: pkg.manager,
        name: pkg.name,
    }))

    if (!targets.length) return

    actionLoading.value = 'bulk-uninstall'
    startLoadingBar()

    try {
        const result = await batchUninstallPackages(targets)
        if (!result.success) throw new Error(result.message)
        toast.success(t('packages.operationCompleted'), {
            description: result.message,
        })
        // 操作成功后重新加载数据
        await reloadPackages()
        packages.value = await loadCachedPackages()
        rowSelection.value = {}
    } catch (error) {
        toast.error(t('packages.requestFailed'), {
            description: t('packages.bulkRemoveError', { error: String(error) }),
        })
    } finally {
        actionLoading.value = null
        finishLoadingBar()
    }
}

async function handleUpgrade(pkg: Package) {
    actionLoading.value = `upgrade:${pkg.manager}:${pkg.name}`
    startLoadingBar()

    try {
        const result = await upgradePackage(pkg.manager, pkg.name)
        if (!result.success) throw new Error(result.message)
        toast.success(t('packages.operationCompleted'), {
            description: result.message,
        })
        // 操作成功后重新加载数据
        await reloadPackages()
        packages.value = await loadCachedPackages()
        rowSelection.value = {}
    } catch (error) {
        toast.error(t('packages.requestFailed'), {
            description: t('packages.upgradeError', { name: pkg.name, error: String(error) }),
        })
    } finally {
        actionLoading.value = null
        finishLoadingBar()
    }
}

async function handleUninstall(pkg: Package) {
    actionLoading.value = `uninstall:${pkg.manager}:${pkg.name}`
    startLoadingBar()

    try {
        const result = await uninstallPackage(pkg.manager, pkg.name)
        if (!result.success) throw new Error(result.message)
        toast.success(t('packages.operationCompleted'), {
            description: result.message,
        })
        // 操作成功后重新加载数据
        await reloadPackages()
        packages.value = await loadCachedPackages()
        rowSelection.value = {}
    } catch (error) {
        toast.error(t('packages.requestFailed'), {
            description: t('packages.uninstallError', {
                name: pkg.name,
                error: String(error),
            }),
        })
    } finally {
        actionLoading.value = null
        finishLoadingBar()
    }
}

onMounted(() => {
    void ensureManagersLoaded()
    void loadPackagesOnMount()
})
</script>

<template>
    <div class="h-full overflow-auto">
        <div class="mx-auto max-w-6xl px-4 py-4 text-[13px]">
            <section>
                <h1 class="text-[28px] font-semibold tracking-tight">{{ t('packages.title') }}</h1>
            </section>

            <section class="mt-7">
                <div class="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
                    <div
                        class="flex flex-1 flex-col gap-3 sm:flex-row sm:items-center sm:justify-start"
                    >
                        <div class="relative w-full sm:max-w-sm">
                            <Search
                                class="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-[hsl(var(--muted-foreground))]"
                            />
                            <Input
                                v-model="searchQuery"
                                class="h-9 rounded-md pl-9 text-[13px]"
                                :placeholder="t('packages.searchPlaceholder')"
                            />
                        </div>

                        <select
                            v-model="filterManager"
                            class="flex h-9 rounded-md border border-[hsl(var(--input))] bg-transparent px-3 text-[13px] outline-none focus-visible:border-[hsl(var(--ring))] focus-visible:ring-[3px] focus-visible:ring-[hsl(var(--ring)/0.2)]"
                        >
                            <option value="all">{{ t('packages.allManagers') }}</option>
                            <option
                                v-for="manager in enabledManagers"
                                :key="manager.id"
                                :value="manager.id"
                            >
                                {{ manager.name }}
                            </option>
                        </select>

                        <Button
                            variant="outline"
                            :disabled="loading"
                            @click="loadPackages"
                            class="justify-right"
                        >
                            <RefreshCcwIcon class="size-4" :class="{ 'animate-spin': loading }" />
                            {{ loading ? t('common.refreshing') : t('common.refresh') }}
                        </Button>
                        <Button
                            class="justify-right"
                            v-if="hasSelectedPackages"
                            variant="outline"
                            :disabled="!hasSelectedOutdatedPackages || actionLoading !== null"
                            @click="handleBulkUpgrade"
                        >
                            <ArrowUpCircle class="size-4" />
                            {{ t('packages.bulkUpdate') }}
                        </Button>
                        <Button
                            v-if="hasSelectedPackages"
                            variant="destructive"
                            class="justify-right"
                            :disabled="actionLoading !== null"
                            @click="handleBulkUninstall"
                        >
                            <Trash2 class="size-4" />
                            {{ t('packages.bulkRemove') }}
                        </Button>
                    </div>
                </div>

                <div class="mt-3 flex flex-wrap gap-2">
                    <Badge variant="outline" class="rounded-sm">
                        {{ t('common.visibleCount', { count: filteredPackages.length }) }}
                    </Badge>
                    <Badge v-if="hasSelectedPackages" variant="outline" class="rounded-sm">
                        {{ t('common.selectedCount', { count: selectedPackages.length }) }}
                    </Badge>
                </div>

                <div class="mt-5">
                    <PackagesDataTable
                        :columns="columns"
                        :data="filteredPackages"
                        v-model:row-selection="rowSelection"
                        :empty-text="
                            loading ? t('packages.loadingPackages') : t('packages.noPackagesFound')
                        "
                    />
                </div>
            </section>
        </div>
    </div>
</template>
