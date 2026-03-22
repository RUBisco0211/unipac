<script setup lang="ts">
import {
    FlexRender,
    getCoreRowModel,
    useVueTable,
    type ColumnDef,
    type RowSelectionState,
} from '@tanstack/vue-table'
import { useI18n } from 'vue-i18n'
import type { Package } from '@/model/types'
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from '@/components/ui/table'

const props = withDefaults(
    defineProps<{
        columns: ColumnDef<Package, unknown>[]
        data: Package[]
        emptyText?: string
        rowSelection?: RowSelectionState
    }>(),
    {
        rowSelection: () => ({}),
    }
)

const emit = defineEmits<{
    'update:rowSelection': [value: RowSelectionState]
}>()

const { t } = useI18n()

const table = useVueTable({
    get data() {
        return props.data
    },
    get columns() {
        return props.columns
    },
    getRowId: row => `${row.manager}:${row.name}`,
    enableRowSelection: true,
    state: {
        get rowSelection() {
            return props.rowSelection
        },
    },
    onRowSelectionChange: updater => {
        const nextValue = typeof updater === 'function' ? updater(props.rowSelection) : updater
        emit('update:rowSelection', nextValue)
    },
    getCoreRowModel: getCoreRowModel(),
})
</script>

<template>
    <div class="bg-transparent">
        <Table>
            <TableHeader>
                <TableRow
                    v-for="headerGroup in table.getHeaderGroups()"
                    :key="headerGroup.id"
                    class="hover:bg-transparent"
                >
                    <TableHead
                        v-for="header in headerGroup.headers"
                        :key="header.id"
                        :class="header.column.id === 'actions' ? 'text-right' : ''"
                    >
                        <FlexRender
                            v-if="!header.isPlaceholder"
                            :render="header.column.columnDef.header"
                            :props="header.getContext()"
                        />
                    </TableHead>
                </TableRow>
            </TableHeader>
            <TableBody>
                <template v-if="table.getRowModel().rows.length">
                    <TableRow
                        v-for="row in table.getRowModel().rows"
                        :key="row.id"
                        :data-state="row.getIsSelected() ? 'selected' : undefined"
                    >
                        <TableCell v-for="cell in row.getVisibleCells()" :key="cell.id">
                            <FlexRender
                                :render="cell.column.columnDef.cell"
                                :props="cell.getContext()"
                            />
                        </TableCell>
                    </TableRow>
                </template>
                <TableRow v-else>
                    <TableCell
                        :colspan="columns.length"
                        class="h-16 text-center text-[hsl(var(--muted-foreground))]"
                    >
                        {{ emptyText || t('table.noResults') }}
                    </TableCell>
                </TableRow>
            </TableBody>
        </Table>
    </div>
</template>
