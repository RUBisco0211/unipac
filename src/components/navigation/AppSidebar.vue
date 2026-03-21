<script setup lang="ts">
const items = [
    {
        label: '已安装',
        icon: 'pi pi-box',
        to: '/packages',
    },
    {
        label: '搜索安装',
        icon: 'pi pi-search',
        to: '/search',
    },
    {
        label: '设置',
        icon: 'pi pi-cog',
        to: '/settings',
    },
]
</script>

<template>
    <aside class="sidebar">
        <Card class="brand-card">
            <template #title>UniPac</template>
            <template #subtitle>Global Package Hub</template>
            <div class="brand-meta">
                <Avatar label="U" shape="circle" class="brand-avatar" />
                <span>统一管理 Homebrew / npm / pip / cargo</span>
            </div>
        </Card>

        <PanelMenu :model="items" />
    </aside>
</template>

<style scoped>
.sidebar {
    padding: 24px 16px;
    display: flex;
    flex-direction: column;
    gap: 16px;
    background: var(--surface-section);
    border-right: 1px solid var(--surface-border);
}

.brand-card :deep(.p-card-body) {
    gap: 8px;
}

.brand-meta {
    display: flex;
    align-items: center;
    gap: 10px;
    color: var(--text-color-secondary);
}

.brand-avatar {
    background: var(--primary-color);
    color: var(--primary-color-text);
}

.sidebar-footnote p {
    margin: 0;
    color: var(--text-color-secondary);
    font-size: 13px;
    line-height: 1.5;
}

@media (max-width: 1024px) {
    .sidebar {
        border-right: none;
        border-bottom: 1px solid var(--surface-border);
    }
}
</style>
