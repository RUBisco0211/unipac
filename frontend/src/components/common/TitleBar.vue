<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowLeft, ArrowRight, Box, PanelLeftClose, PanelLeftOpen } from 'lucide-vue-next'

import { Button } from '@/components/ui/button'
import { appWindow } from '@/composables/useWindow'
import { useHistory } from '@/composables/useHistory'

const { canGoBack, canGoForward, goBack,goForward } = useHistory()

const { type = 'system', collapsed } = defineProps<{
    type?: 'system' | 'hidden'
    collapsed: boolean
}>()

const emit = defineEmits<{
    'toggle-sidebar': []
}>()

const titlebarButtonClass =
    'size-8 rounded-xl text-[hsl(var(--muted-foreground))] hover:bg-[hsl(var(--accent))] hover:text-[hsl(var(--foreground))] disabled:text-[hsl(var(--muted-foreground)/0.45)] disabled:hover:bg-transparent'

const router = useRouter()
const isFullscreen = ref(false)

const leftInset = computed(() => (isFullscreen.value ? 0 : 72))

async function syncWindowState() {
    isFullscreen.value = await appWindow.isFullscreen()
}

const handleDoubleClick = async (event: MouseEvent) => {
    const target = event.target as HTMLElement
    if (target.closest('.no-drag') || event.button !== 0) {
        return
    }
    appWindow.toggleMaximize()
}

onMounted(async () => {
    await syncWindowState()
})
</script>

<template>
    <div
        v-if="type === 'system'"
        class="title-bar shrink-0 border-b border-[hsl(var(--border))] bg-[hsl(var(--background))]"
        @dblclick="handleDoubleClick"
    >
        <div class="relative flex h-8 items-center px-4">
            <div
                class="title-bar-controls no-drag flex items-center"
                :style="{ marginLeft: `${leftInset}px` }"
            >
                <Button
                    variant="ghost"
                    size="icon"
                    :class="titlebarButtonClass"
                    :disabled="!canGoBack"
                    @click="goBack"
                >
                    <ArrowLeft class="size-4" />
                </Button>

                <Button
                    variant="ghost"
                    size="icon"
                    :class="titlebarButtonClass"
                    :disabled="!canGoForward"
                    @click="goForward"
                >
                    <ArrowRight class="size-4" />
                </Button>

                <Button
                    variant="ghost"
                    size="icon"
                    :class="titlebarButtonClass"
                    @click="emit('toggle-sidebar')"
                >
                    <PanelLeftOpen v-if="collapsed" class="size-4" />
                    <PanelLeftClose v-else class="size-4" />
                </Button>
            </div>

            <div
                class="pointer-events-none absolute inset-x-0 flex items-center justify-center gap-2 text-[13px] font-medium text-[hsl(var(--foreground))]"
            >
                <Box class="size-4" />
                <b>UniPac</b>
            </div>
        </div>
    </div>
</template>

<style>
.title-bar {
    --wails-draggable: drag;
}

.title-bar button {
    --wails-draggable: no-drag;
}

.title-bar-controls {
    transition: margin-left 180ms ease;
}
</style>
