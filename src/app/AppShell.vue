<script setup lang="ts">
import { computed } from 'vue'
import { RouterView, useRoute } from 'vue-router'
import AppSidebar from '@/components/navigation/AppSidebar.vue'

const route = useRoute()

const pageTitle = computed(() => {
    return (route.meta.title as string | undefined) ?? 'UniPac'
})

const pageDescription = computed(() => {
    return (
        (route.meta.description as string | undefined) ??
        '统一管理 Homebrew、npm、pip 和 cargo 的全局工具。'
    )
})
</script>

<template>
    <div class="app-shell">
        <AppSidebar />

        <div class="app-shell__main">
            <Card class="app-shell__header">
                <template #title>{{ pageTitle }}</template>
                <template #subtitle>Global Package Workspace</template>
                <p class="app-shell__description">{{ pageDescription }}</p>
            </Card>

            <main class="app-shell__content">
                <RouterView />
            </main>
        </div>
    </div>
</template>

<style scoped>
.app-shell {
    min-height: 100vh;
    display: grid;
    grid-template-columns: 280px minmax(0, 1fr);
    background: var(--surface-ground);
}

.app-shell__main {
    min-width: 0;
    padding: 24px;
}

.app-shell__header :deep(.p-card-body) {
    gap: 6px;
}

.app-shell__description {
    margin: 0;
    color: var(--text-color-secondary);
}

.app-shell__content {
    padding: 20px 0 0;
}

@media (max-width: 1024px) {
    .app-shell {
        grid-template-columns: 1fr;
    }

    .app-shell__main {
        padding: 16px;
    }
}
</style>
