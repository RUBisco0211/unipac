<script setup lang="ts">
import { Box,Sidebar, SidebarIcon } from 'lucide-vue-next'
import { getCurrentWindow } from '@tauri-apps/api/window'

const { type = 'system' } = defineProps<{
    type: 'system' |  'hidden'
}>()
const appWindow = getCurrentWindow()

const handleDoubleClick = async (e:MouseEvent) => {
        const target = e.target as HTMLElement
    if (target.closest('.no-drag')) {
        return
    }
    // 只在左键按下时拖拽
    if (e.button !== 0) return;
    await appWindow.maximize();
}

const startDrag = (e: MouseEvent) => {
    const target = e.target as HTMLElement
    if (target.closest('.no-drag')) {
        return
    }
    // 只在左键按下时拖拽
    if (e.button !== 0) return;
    
    // 调用 Tauri 的 startDragging API
    appWindow.startDragging()

    // 阻止默认行为，避免与文本选择等冲突
    e.preventDefault()
}


</script>
<template>
    <div
        v-if="type === 'system'"
        @mousedown="startDrag"
        @dblclick="handleDoubleClick"
        class="title-bar flex items-center px-4 shrink-0 select-none"
    >
        <div class="no-drag">
            <Button variant="ghost" size="icon" class="rounded ml-20">
                <SidebarIcon class="h-4" />
            </Button>
        </div>
        <div
            class="theme-text-secondary flex-1 flex justify-center items-center text-sm font-semibold"
        >
            <Box class="w-4 h-4 mr-2" />
            UniPac
        </div>
    </div>
</template>
<style>
.title-bar {
    height: 48px;
    -webkit-app-region: drag;
}
.title-bar button {
    -webkit-app-region: no-drag;
}
</style>
