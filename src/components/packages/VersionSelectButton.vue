<script setup lang="ts">
import { Download, Loader2 } from 'lucide-vue-next'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog'
import { getPackageVersions } from '@/lib/api'
import type { Package } from '@/model/types'

type InstallCallback = (pkg: Package, version?: string) => void

interface Props {
    package: Package
    disabled?: boolean
    loadingKey?: string | null
    onInstall: InstallCallback
}

const props = withDefaults(defineProps<Props>(), {
    disabled: false,
    loadingKey: null,
})

const { t } = useI18n()

const isDialogOpen = ref(false)
const isLoading = ref(false)
const versions = ref<string[]>([])
const versionsError = ref<string | null>(null)
const selectedVersion = ref<string | undefined>(undefined)

const installKey = computed(() => `install:${props.package.manager}:${props.package.name}`)
const isInstalling = computed(() => props.loadingKey === installKey.value)

const buttonText = computed(() => {
    if (isInstalling.value) {
        return t('actions.installing')
    }
    return t('actions.install')
})

async function loadVersions() {
    if (versions.value.length > 0) {
        return
    }

    isLoading.value = true
    versionsError.value = null

    try {
        versions.value = await getPackageVersions(props.package.manager, props.package.name)
        selectedVersion.value = versions.value[0]
    } catch (error) {
        versionsError.value = String(error)
        versions.value = []
    } finally {
        isLoading.value = false
    }
}

function openDialog() {
    if (props.disabled || isInstalling.value) return
    isDialogOpen.value = true
    void loadVersions()
}

function confirmInstall(pack: Package, version?: string) {
    props.onInstall(pack, version)
    isDialogOpen.value = false
}
</script>

<template>
    <Button :disabled="disabled || isInstalling" size="sm" variant="ghost" @click="openDialog">
        <Download v-if="!isInstalling" class="size-4" />
        <Loader2 v-else class="size-4 animate-spin" />
        {{ buttonText }}
    </Button>

    <Dialog :open="isDialogOpen" @update:open="value => (isDialogOpen = value)">
        <DialogContent class="max-w-md p-0">
            <DialogHeader>
                <DialogTitle>{{ t('search.chooseVersion') }}</DialogTitle>
                <DialogDescription>{{ props.package.name }}</DialogDescription>
            </DialogHeader>

            <div class="space-y-3 px-4 py-4">
                <div
                    v-if="isLoading"
                    class="flex items-center gap-2 text-sm text-[hsl(var(--muted-foreground))]"
                >
                    <Loader2 class="size-4 animate-spin" />
                    {{ t('search.loadingVersions') }}
                </div>

                <div v-else-if="versionsError" class="text-sm text-[hsl(var(--destructive))]">
                    {{ versionsError }}
                </div>

                <div
                    v-else-if="versions.length === 0"
                    class="text-sm text-[hsl(var(--muted-foreground))]"
                >
                    {{ t('search.noVersions') }}
                </div>

                <template v-else>
                    <p class="text-xs text-[hsl(var(--muted-foreground))]">
                        {{ t('search.selectedVersion') }}:
                        <span class="font-medium text-[hsl(var(--foreground))]">
                            {{ selectedVersion || t('search.latestVersion') }}
                        </span>
                    </p>

                    <div
                        class="max-h-64 overflow-auto rounded-md border border-[hsl(var(--border))]"
                    >
                        <button
                            v-for="version in versions"
                            :key="version"
                            type="button"
                            class="flex w-full items-center justify-between border-b border-[hsl(var(--border)/0.45)] px-3 py-2 text-left text-sm last:border-b-0 hover:bg-[hsl(var(--accent)/0.65)]"
                            :class="
                                selectedVersion === version
                                    ? 'bg-[hsl(var(--accent))] text-[hsl(var(--accent-foreground))]'
                                    : ''
                            "
                            @click="selectedVersion = version"
                        >
                            <span>{{ version }}</span>
                        </button>
                    </div>
                </template>
            </div>

            <DialogFooter>
                <Button
                    :disabled="isLoading || !selectedVersion || isInstalling"
                    variant="default"
                    @click="confirmInstall($props.package)"
                >
                    <Loader2 v-if="isInstalling" class="size-4 animate-spin" />
                    <Download v-else class="size-4" />
                    {{ isInstalling ? t('actions.installing') : t('actions.install') }}
                </Button>
                <Button
                    variant="secondary"
                    :disabled="isLoading || !selectedVersion || isInstalling"
                    @click="confirmInstall($props.package, selectedVersion)"
                >
                    <Loader2 v-if="isInstalling" class="size-4 animate-spin" />
                    <Download v-else class="size-4" />
                    {{
                        isInstalling
                            ? t('actions.installing')
                            : `${t('search.installSelected')} ${selectedVersion ?? ''}`
                    }}
                </Button>
            </DialogFooter>
        </DialogContent>
    </Dialog>
</template>
