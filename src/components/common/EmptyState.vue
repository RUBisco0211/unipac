<script setup lang="ts">
defineProps<{
    title: string
    description: string
}>()
</script>

<template>
    <Card class="empty-state">
        <template #title>{{ title }}</template>
        <template #content>
            <p>{{ description }}</p>
        </template>
    </Card>
</template>

<style scoped>
.empty-state :deep(.p-card-body) {
    text-align: center;
    gap: 8px;
}

p {
    margin: 0;
    color: var(--text-color-secondary);
}
</style>
