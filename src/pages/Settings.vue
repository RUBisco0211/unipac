<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { listManagers } from '@/lib/api'
import { enabledCapabilities } from '@/lib/format'
import type { ManagerInfo } from '@/model/types'

const loading = ref(false)
const managers = ref<ManagerInfo[]>([])
const { t } = useI18n()

const enabledCount = computed(() => managers.value.filter(manager => manager.enabled).length)

function capabilityLabel(capability: string) {
    return t(`capabilities.${capability}`)
}

async function loadManagers() {
    loading.value = true

    try {
        managers.value = await listManagers()
    } catch (error) {
        toast.error(t('settings.loadErrorTitle'), {
            description: t('settings.loadError', { error: String(error) }),
        })
    } finally {
        loading.value = false
    }
}

onMounted(() => {
    void loadManagers()
})
</script>

<template>
    <div class="h-full overflow-auto">
        <div class="mx-auto max-w-6xl px-4 py-4 text-[13px]">
            <section>
                <h1 class="text-[28px] font-semibold tracking-tight">{{ t('settings.title') }}</h1>
            </section>

            <section class="mt-7 border-b border-[hsl(var(--border)/0.6)] pb-7">
                <div class="grid gap-6 md:grid-cols-[minmax(0,1fr)_220px]">
                    <div>
                        <h2 class="text-[19px] font-semibold">{{ t('settings.registryTitle') }}</h2>
                    </div>

                    <div class="space-y-2 text-[13px]">
                        <div
                            class="flex items-baseline justify-between border-b border-[hsl(var(--border)/0.6)] pb-2"
                        >
                            <span class="text-[hsl(var(--muted-foreground))]">{{
                                t('settings.enabled')
                            }}</span>
                            <span class="text-[19px] font-semibold">
                                {{ enabledCount }}/{{ managers.length }}
                            </span>
                        </div>
                    </div>
                </div>

                <div class="mt-5 flex gap-2">
                    <Button variant="outline" :disabled="loading" @click="loadManagers">
                        {{ loading ? t('common.refreshing') : t('settings.refreshManagers') }}
                    </Button>
                </div>
            </section>

            <section class="mt-7">
                <h2 class="text-[19px] font-semibold">{{ t('settings.managersTitle') }}</h2>

                <div class="mt-5 overflow-hidden">
                    <table class="w-full text-[12px]">
                        <thead
                            class="border-b border-[hsl(var(--border)/0.45)] text-left text-[hsl(var(--muted-foreground))]"
                        >
                            <tr>
                                <th class="h-8 px-4 font-medium">
                                    {{ t('settings.managerColumn') }}
                                </th>
                                <th class="h-8 px-4 font-medium">
                                    {{ t('settings.statusColumn') }}
                                </th>
                                <th class="h-8 px-4 font-medium">
                                    {{ t('settings.capabilitiesColumn') }}
                                </th>
                            </tr>
                        </thead>
                        <tbody>
                            <tr
                                v-for="manager in managers"
                                :key="manager.id"
                                class="border-b border-[hsl(var(--border)/0.45)] last:border-b-0"
                            >
                                <td class="px-4 py-2.5 align-top">
                                    <div class="font-medium">{{ manager.name }}</div>
                                    <div class="mt-1 text-xs text-[hsl(var(--muted-foreground))]">
                                        id: {{ manager.id }}
                                    </div>
                                </td>
                                <td class="px-4 py-2.5 align-top">
                                    <Badge
                                        :variant="manager.enabled ? 'default' : 'outline'"
                                        class="rounded-sm"
                                    >
                                        {{
                                            manager.enabled
                                                ? t('settings.enabledState')
                                                : t('settings.disabledState')
                                        }}
                                    </Badge>
                                </td>
                                <td class="px-4 py-2.5 align-top">
                                    <div
                                        v-if="enabledCapabilities(manager.capabilities).length"
                                        class="flex flex-wrap gap-2"
                                    >
                                        <Badge
                                            v-for="capability in enabledCapabilities(
                                                manager.capabilities
                                            )"
                                            :key="capability"
                                            variant="secondary"
                                            class="rounded-sm"
                                        >
                                            {{ capabilityLabel(capability) }}
                                        </Badge>
                                    </div>
                                    <span v-else class="text-[hsl(var(--muted-foreground))]">
                                        {{ t('settings.noCapabilities') }}
                                    </span>
                                </td>
                            </tr>
                            <tr v-if="!loading && managers.length === 0">
                                <td
                                    colspan="3"
                                    class="px-4 py-7 text-center text-[hsl(var(--muted-foreground))]"
                                >
                                    {{ t('settings.noManagers') }}
                                </td>
                            </tr>
                        </tbody>
                    </table>
                </div>
            </section>
        </div>
    </div>
</template>
