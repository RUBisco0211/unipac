import { computed, ref } from 'vue'
import { listManagers } from '@/lib/api'
import type { ManagerInfo, ManagerType } from '@/model/types'

const managers = ref<ManagerInfo[]>([])
const loaded = ref(false)
const loading = ref(false)

export async function ensureManagersLoaded(force = false) {
    if (loading.value) return
    if (loaded.value && !force) return

    loading.value = true
    try {
        managers.value = await listManagers()
        loaded.value = true
    } finally {
        loading.value = false
    }
}

export function useManagers() {
    const enabledManagers = computed(() =>
        managers.value.filter(manager => manager.enabled && manager.capabilities.list)
    )

    const managerNameMap = computed<Record<string, string>>(() =>
        Object.fromEntries(managers.value.map(manager => [manager.id, manager.name]))
    )

    const hasManager = (manager: ManagerType | string) =>
        managers.value.some(item => item.id === manager && item.enabled)

    return {
        managers,
        enabledManagers,
        managerNameMap,
        hasManager,
        loadingManagers: loading,
    }
}
