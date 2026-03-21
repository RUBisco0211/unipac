<script setup lang="ts">
import { managerLabelMap } from '@/lib/format'
import type { ManagerType } from '@/model/types'

defineProps<{
    manager: ManagerType
}>()
</script>

<template>
    <Tag :value="managerLabelMap[manager]" severity="info" rounded />
</template>
