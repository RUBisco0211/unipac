<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowLeft, ArrowRight, Box, PanelLeftClose, PanelLeftOpen } from 'lucide-vue-next'
import { getCurrentWindow } from '@tauri-apps/api/window'
import { Button } from '@/components/ui/button'

const props = withDefaults(
    defineProps<{
        type?: 'system' | 'hidden'
        collapsed?: boolean
    }>(),
    {
        type: 'system',
        collapsed: false,
    }
)

const emit = defineEmits<{
    'toggle-sidebar': []
}>()

const router = useRouter()
const appWindow = getCurrentWindow()
const isFullscreen = ref(false)
const historyState = ref<Record<string, unknown>>({})

const leftInset = computed(() => (isFullscreen.value ? 0 : 72))
const canGoBack = computed(() => Boolean(historyState.value.back))
const canGoForward = computed(() => Boolean(historyState.value.forward))

let removeResizeListener: (() => void) | null = null
let removeAfterEachHook: (() => void) | null = null
let removePopStateListener: (() => void) | null = null

function syncHistoryState() {
    historyState.value = (window.history.state as Record<string, unknown> | null) ?? {}
}

async function syncWindowState() {
    isFullscreen.value = await appWindow.isFullscreen()
}

const handleDoubleClick = async (event: MouseEvent) => {
    const target = event.target as HTMLElement
    if (target.closest('.no-drag') || event.button !== 0) {
        return
    }

    await appWindow.toggleMaximize()
}

const startDrag = async (event: MouseEvent) => {
    const target = event.target as HTMLElement
    if (target.closest('.no-drag') || event.button !== 0) {
        return
    }

    await appWindow.startDragging()
    event.preventDefault()
}

function goBack() {
    if (!canGoBack.value) return
    router.back()
}

function goForward() {
    if (!canGoForward.value) return
    router.forward()
}

onMounted(async () => {
    await syncWindowState()
    syncHistoryState()

    removeResizeListener = await appWindow.onResized(() => {
        void syncWindowState()
    })

    removeAfterEachHook = router.afterEach(() => {
        syncHistoryState()
    })

    const handlePopState = () => {
        syncHistoryState()
    }

    window.addEventListener('popstate', handlePopState)
    removePopStateListener = () => {
        window.removeEventListener('popstate', handlePopState)
    }
})

onBeforeUnmount(() => {
    removeResizeListener?.()
    removeAfterEachHook?.()
    removePopStateListener?.()
})
</script>

<template>
    <div
        v-if="props.type === 'system'"
        class="title-bar shrink-0 border-b border-[hsl(var(--border))] bg-[hsl(var(--background))]"
        @mousedown="startDrag"
        @dblclick="handleDoubleClick"
    >
        <div class="relative flex h-9 items-center px-4">
            <div
                class="title-bar-controls no-drag flex items-center gap-1.5"
                :style="{ marginLeft: `${leftInset}px` }"
            >
                <Button
                    variant="ghost"
                    size="icon"
                    class="size-6 rounded-md text-[hsl(var(--muted-foreground))] hover:bg-[hsl(var(--accent))] hover:text-[hsl(var(--foreground))] disabled:text-[hsl(var(--muted-foreground)/0.45)] disabled:hover:bg-transparent"
                    :disabled="!canGoBack"
                    @click="goBack"
                >
                    <ArrowLeft class="size-4" />
                </Button>

                <Button
                    variant="ghost"
                    size="icon"
                    class="size-6 rounded-md text-[hsl(var(--muted-foreground))] hover:bg-[hsl(var(--accent))] hover:text-[hsl(var(--foreground))] disabled:text-[hsl(var(--muted-foreground)/0.45)] disabled:hover:bg-transparent"
                    :disabled="!canGoForward"
                    @click="goForward"
                >
                    <ArrowRight class="size-4" />
                </Button>

                <Button
                    variant="ghost"
                    size="icon"
                    class="size-6 rounded-md text-[hsl(var(--muted-foreground))] hover:bg-[hsl(var(--accent))] hover:text-[hsl(var(--foreground))]"
                    @click="emit('toggle-sidebar')"
                >
                    <PanelLeftOpen v-if="props.collapsed" class="size-4" />
                    <PanelLeftClose v-else class="size-4" />
                </Button>
            </div>

            <div
                class="pointer-events-none absolute inset-x-0 flex items-center justify-center gap-2 text-[13px] font-medium text-[hsl(var(--foreground))]"
            >
                <Box class="size-4" />
                <span>UniPac</span>
            </div>
        </div>
    </div>
</template>

<style>
.title-bar {
    -webkit-app-region: drag;
}

.title-bar button {
    -webkit-app-region: no-drag;
}

.title-bar-controls {
    transition: margin-left 180ms ease;
}
</style>
