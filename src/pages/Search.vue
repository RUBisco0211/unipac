<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Search } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import PackagesDataTable from '@/components/packages/PackagesDataTable.vue'
import { createPackageColumns } from '@/components/packages/packages-columns'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { finishLoadingBar, startLoadingBar } from '@/composables/useLoadingBar'
import { ensureManagersLoaded, useManagers } from '@/composables/useManagers'
import { installPackage, searchPackages } from '@/lib/api'
import type { Package } from '@/model/types'

const searchQuery = ref('')
const versionInput = ref('')
const isSearching = ref(false)
const hasSearched = ref(false)
const installingKey = ref<string | null>(null)
const searchResults = ref<Package[]>([])
const { t, locale } = useI18n()
const { managerNameMap } = useManagers()

const columns = computed(() => {
    locale.value
    return createPackageColumns({
        mode: 'search',
        loadingKey: installingKey.value,
        managerLabels: managerNameMap.value,
        onInstall: pkg => {
            void handleInstall(pkg)
        },
    })
})

async function handleSearch() {
    if (!searchQuery.value.trim()) return

    isSearching.value = true
    hasSearched.value = true
    startLoadingBar()

    try {
        searchResults.value = await searchPackages(searchQuery.value.trim())
    } catch (error) {
        toast.error(t('search.errorTitle'), {
            description: t('search.searchError', { error: String(error) }),
        })
        searchResults.value = []
    } finally {
        isSearching.value = false
        finishLoadingBar()
    }
}

async function handleInstall(pkg: Package) {
    installingKey.value = `install:${pkg.manager}:${pkg.name}`

    try {
        const options = versionInput.value.trim() ? { version: versionInput.value.trim() } : undefined
        const result = await installPackage(pkg.manager, pkg.name, options)
        if (!result.success) throw new Error(result.message)
        toast.success(t('search.startedTitle'), {
            description: result.message,
        })
    } catch (error) {
        toast.error(t('search.errorTitle'), {
            description: t('search.installError', { name: pkg.name, error: String(error) }),
        })
    } finally {
        installingKey.value = null
    }
}

void ensureManagersLoaded()
</script>

<template>
    <div class="h-full overflow-auto">
        <div class="mx-auto max-w-6xl px-4 py-4 text-[13px]">
            <section>
                <h1 class="text-[28px] font-semibold tracking-tight">{{ t('search.title') }}</h1>
            </section>

            <section class="mt-7 border-b border-[hsl(var(--border)/0.6)] pb-7">
                <form class="flex flex-col gap-3 sm:flex-row" @submit.prevent="handleSearch">
                    <div class="relative flex-1">
                        <Search
                            class="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-[hsl(var(--muted-foreground))]"
                        />
                        <Input
                            v-model="searchQuery"
                            class="h-9 rounded-md pl-9 text-[13px]"
                            :placeholder="t('search.placeholder')"
                        />
                    </div>
                    <Button type="submit" class="h-9 px-4 text-[13px]" :disabled="isSearching">
                        <Search class="size-4" />
                        {{ isSearching ? t('search.searching') : t('search.action') }}
                    </Button>
                </form>
            </section>

            <section v-if="hasSearched" class="mt-7">
                <div class="flex flex-wrap items-center gap-2">
                    <h2 class="text-[19px] font-semibold">{{ t('search.resultsTitle') }}</h2>
                    <Badge variant="outline" class="rounded-sm">
                        {{ t('search.matches', { count: searchResults.length }) }}
                    </Badge>
                </div>
                <div class="mt-5">
                    <PackagesDataTable
                        :columns="columns"
                        :data="searchResults"
                        :empty-text="
                            isSearching
                                ? t('search.searchingPackages')
                                : t('search.noMatches', { query: searchQuery })
                        "
                    />
                </div>
            </section>

            <section v-else class="mt-7 py-8 text-center">
                <p class="text-[13px] text-[hsl(var(--muted-foreground))]">
                    {{ t('search.emptyHint') }}
                </p>
            </section>
        </div>
    </div>
</template>
