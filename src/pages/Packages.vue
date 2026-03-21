<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Search, RefreshCw, Trash2, ArrowUpCircle, AlertCircle } from 'lucide-vue-next'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import Badge from '@/components/ui/Badge.vue'
import { listInstalledPackages, uninstallPackage, upgradePackage } from '@/lib/api'
import { countInstalledSummary, managerLabelMap } from '@/lib/format'
import type { ManagerType, Package } from '@/model/types'

const searchQuery = ref('')
const filterManager = ref<ManagerType | 'all'>('all')
const loading = ref(false)
const actionLoading = ref<string | null>(null)
const errorMessage = ref('')
const feedbackMessage = ref('')
const packages = ref<Package[]>([])

const summary = computed(() => countInstalledSummary(packages.value))

const filteredPackages = computed(() => {
    const query = searchQuery.value.trim().toLowerCase()

    return packages.value.filter(pkg => {
        const matchesSearch =
            query.length === 0 ||
            pkg.name.toLowerCase().includes(query) ||
            pkg.description?.toLowerCase().includes(query) ||
            pkg.fullname?.toLowerCase().includes(query)
        const matchesManager = filterManager.value === 'all' || pkg.manager === filterManager.value

        return matchesSearch && matchesManager
    })
})

const getManagerColor = (manager: ManagerType) => {
    const colors: Record<ManagerType, string> = {
        npm: 'manager-badge-npm',
        brew: 'manager-badge-brew',
        pip: 'manager-badge-pip',
        cargo: 'manager-badge-cargo',
    }

    return colors[manager]
}

async function loadPackages() {
    loading.value = true
    errorMessage.value = ''

    try {
        packages.value = await listInstalledPackages()
    } catch (error) {
        errorMessage.value = `Failed to load installed packages: ${String(error)}`
    } finally {
        loading.value = false
    }
}

async function handleUpgrade(pkg: Package) {
    actionLoading.value = `upgrade:${pkg.manager}:${pkg.name}`
    feedbackMessage.value = ''
    errorMessage.value = ''

    try {
        const result = await upgradePackage(pkg.manager, pkg.name)
        if (!result.success) {
            throw new Error(result.message)
        }
        feedbackMessage.value = result.message
        await loadPackages()
    } catch (error) {
        errorMessage.value = `Failed to upgrade ${pkg.name}: ${String(error)}`
    } finally {
        actionLoading.value = null
    }
}

async function handleUninstall(pkg: Package) {
    actionLoading.value = `uninstall:${pkg.manager}:${pkg.name}`
    feedbackMessage.value = ''
    errorMessage.value = ''

    try {
        const result = await uninstallPackage(pkg.manager, pkg.name)
        if (!result.success) {
            throw new Error(result.message)
        }
        feedbackMessage.value = result.message
        await loadPackages()
    } catch (error) {
        errorMessage.value = `Failed to uninstall ${pkg.name}: ${String(error)}`
    } finally {
        actionLoading.value = null
    }
}

onMounted(() => {
    void loadPackages()
})
</script>

<template>
    <div class="flex-1 flex flex-col h-full">
        <header class="theme-header h-14 flex items-center justify-between px-6 border-b shrink-0">
            <h1 class="theme-text text-lg font-semibold">Installed Packages</h1>
        </header>
        <div class="theme-app p-6 flex-1 overflow-auto">
            <div class="grid grid-cols-1 md:grid-cols-3 gap-4 mb-6">
                <div class="theme-panel-subtle rounded-md border p-4">
                    <p class="theme-text-muted text-sm">Total Packages</p>
                    <p class="theme-text mt-2 text-2xl font-semibold">{{ summary.total }}</p>
                </div>
                <div class="theme-panel-subtle rounded-md border p-4">
                    <p class="theme-text-muted text-sm">GUI Apps</p>
                    <p class="theme-text mt-2 text-2xl font-semibold">{{ summary.gui }}</p>
                </div>
                <div class="theme-panel-subtle rounded-md border p-4">
                    <p class="theme-text-muted text-sm">Outdated</p>
                    <p class="theme-text mt-2 text-2xl font-semibold">{{ summary.outdated }}</p>
                </div>
            </div>

            <div class="flex items-center justify-between mb-6 space-x-4">
                <div class="flex items-center space-x-2 flex-1 max-w-md">
                    <div class="relative flex-1">
                        <Search class="theme-text-muted absolute left-2.5 top-2.5 h-4 w-4" />
                        <Input
                            v-model="searchQuery"
                            placeholder="Search local packages..."
                            class="pl-9"
                        />
                    </div>
                    <select
                        v-model="filterManager"
                        class="theme-panel theme-border theme-text h-9 rounded-md border px-3 py-1 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-[var(--color-focus)]"
                    >
                        <option value="all">All Managers</option>
                        <option value="npm">npm</option>
                        <option value="brew">Homebrew</option>
                        <option value="pip">pip</option>
                        <option value="cargo">cargo</option>
                    </select>
                </div>
                <Button variant="outline" :disabled="loading" @click="loadPackages">
                    <RefreshCw class="w-4 h-4 mr-2" :class="{ 'animate-spin': loading }" />
                    Refresh
                </Button>
            </div>

            <div
                v-if="errorMessage"
                class="theme-alert-danger mb-4 flex items-center rounded-md border px-4 py-3 text-sm"
            >
                <AlertCircle class="mr-2 h-4 w-4 shrink-0" />
                {{ errorMessage }}
            </div>

            <div
                v-if="feedbackMessage"
                class="theme-alert-success mb-4 rounded-md border px-4 py-3 text-sm"
            >
                {{ feedbackMessage }}
            </div>

            <div class="theme-panel theme-border rounded-md border shadow-sm">
                <table class="w-full text-sm text-left">
                    <thead
                        class="theme-text-muted theme-panel-subtle theme-border border-b text-xs"
                    >
                        <tr>
                            <th class="h-10 px-4 font-medium align-middle">Name</th>
                            <th class="h-10 px-4 font-medium align-middle">Version</th>
                            <th class="h-10 px-4 font-medium align-middle">Latest</th>
                            <th class="h-10 px-4 font-medium align-middle">Type</th>
                            <th class="h-10 px-4 font-medium align-middle">Manager</th>
                            <th class="h-10 px-4 font-medium align-middle text-right">Actions</th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr
                            v-for="pkg in filteredPackages"
                            :key="`${pkg.manager}-${pkg.name}`"
                            class="theme-border theme-panel-hover border-b last:border-0 transition-colors"
                        >
                            <td class="p-4 align-middle">
                                <div class="theme-text font-medium">{{ pkg.name }}</div>
                                <div v-if="pkg.description" class="theme-text-muted mt-1 text-xs">
                                    {{ pkg.description }}
                                </div>
                            </td>
                            <td class="theme-text-secondary p-4 align-middle">{{ pkg.version }}</td>
                            <td class="p-4 align-middle">
                                <span
                                    :class="
                                        pkg.version === pkg.latest_version
                                            ? 'theme-text-secondary'
                                            : 'theme-warning-text font-medium'
                                    "
                                >
                                    {{ pkg.latest_version }}
                                </span>
                            </td>
                            <td class="p-4 align-middle">
                                <Badge variant="outline" class="text-xs font-normal">
                                    {{ pkg.is_gui ? 'GUI' : 'CLI' }}
                                </Badge>
                            </td>
                            <td class="p-4 align-middle">
                                <Badge variant="outline" :class="getManagerColor(pkg.manager)">
                                    {{ managerLabelMap[pkg.manager] }}
                                </Badge>
                            </td>
                            <td class="p-4 align-middle text-right space-x-2 whitespace-nowrap">
                                <Button
                                    v-if="pkg.outdated"
                                    variant="outline"
                                    size="sm"
                                    class="h-8"
                                    :disabled="actionLoading !== null"
                                    @click="handleUpgrade(pkg)"
                                >
                                    <ArrowUpCircle class="w-3.5 h-3.5 mr-1.5" />
                                    Update
                                </Button>
                                <Button
                                    variant="ghost"
                                    size="sm"
                                    class="h-8 text-[var(--color-danger-text)] hover:bg-[var(--color-danger-bg)]"
                                    :disabled="actionLoading !== null"
                                    @click="handleUninstall(pkg)"
                                >
                                    <Trash2 class="w-3.5 h-3.5 mr-1.5" />
                                    Uninstall
                                </Button>
                            </td>
                        </tr>
                        <tr v-if="filteredPackages.length === 0">
                            <td colspan="6" class="theme-text-muted p-8 text-center">
                                {{ loading ? 'Loading packages...' : 'No packages found.' }}
                            </td>
                        </tr>
                    </tbody>
                </table>
            </div>
        </div>
    </div>
</template>
