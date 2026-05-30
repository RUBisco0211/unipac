import type { ColumnDef } from '@tanstack/vue-table'
import { ArrowUpCircle, GitBranch, Package as PackageIcon, Tag, Trash2 } from 'lucide-vue-next'
import { h } from 'vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { i18n } from '@/i18n'
import { getManagerHue, managerLabelMap } from '@/lib/format'
import type { Package } from '@/model/types'
import VersionSelectButton from '@/components/packages/VersionSelectButton.vue'

type ColumnActions = {
    mode: 'installed' | 'search'
    loadingKey: string | null
    managerLabels?: Record<string, string>
    selectable?: boolean
    onUpgrade?: (pkg: Package) => void
    onUninstall?: (pkg: Package) => void
    onInstall?: (pkg: Package, version?: string) => void
}

export function createPackageColumns(actions: ColumnActions): ColumnDef<Package>[] {
    const { t } = i18n.global

    const columns: ColumnDef<Package>[] = []

    if (actions.selectable) {
        columns.push({
            id: 'select',
            enableSorting: false,
            header: ({ table }) =>
                h('input', {
                    type: 'checkbox',
                    'aria-label': t('table.select'),
                    checked: table.getIsAllRowsSelected(),
                    indeterminate: table.getIsSomeRowsSelected(),
                    class: 'size-3.5 rounded border border-[hsl(var(--border))]',
                    onChange: table.getToggleAllRowsSelectedHandler(),
                }),
            cell: ({ row }) =>
                h('input', {
                    type: 'checkbox',
                    'aria-label': t('table.select'),
                    checked: row.getIsSelected(),
                    disabled: !row.getCanSelect(),
                    class: 'size-3.5 rounded border border-[hsl(var(--border))]',
                    onChange: row.getToggleSelectedHandler(),
                }),
        })
    }

    columns.push(
        {
            accessorKey: 'name',
            enableSorting: true,
            header: t('table.package'),
            cell: ({ row }) =>
                h('div', { class: 'flex items-center gap-1' }, [
                    h(PackageIcon, { class: 'size-4' }),
                    h('div', { class: 'space-y-1' }, [
                        h(
                            'div',
                            { class: 'font-medium text-[hsl(var(--foreground))]' },
                            row.original.name
                        ),
                        h(
                            'div',
                            {
                                class: 'max-w-[360px] truncate text-xs text-[hsl(var(--muted-foreground))]',
                            },
                            row.original.description ||
                                row.original.fullname ||
                                t('table.noDescription')
                        ),
                    ]),
                ]),
        },
        {
            accessorKey: 'version',
            enableSorting: false,
            header: t('table.installed'),
            cell: ({ row }) =>
                h('div', { class: 'flex items-center gap-1' }, [
                    h(Tag, { class: 'size-4' }),
                    h(
                        'span',
                        { class: 'text-[hsl(var(--muted-foreground))]' },

                        row.original.version || '-'
                    ),
                ]),
        },
        {
            accessorKey: 'latest_version',
            enableSorting: false,
            header: t('table.latest'),
            cell: ({ row }) =>
                h('div', { class: 'flex items-center gap-1' }, [
                    h(GitBranch, { class: 'size-4' }),
                    h(
                        'span',
                        {
                            class: row.original.outdated
                                ? 'font-medium text-[hsl(var(--primary))]'
                                : 'text-[hsl(var(--muted-foreground))]',
                        },

                        row.original.latest_version || row.original.version || '-'
                    ),
                ]),
        },
        {
            id: 'type',
            accessorFn: row => (row.is_gui ? t('packageType.gui') : t('packageType.cli')),
            enableSorting: true,
            sortingFn: 'text',
            header: t('table.type'),
            cell: ({ row }) =>
                h(Badge, { variant: 'outline', class: 'rounded-sm' }, () =>
                    row.original.is_gui ? t('packageType.gui') : t('packageType.cli')
                ),
        },
        {
            id: 'manager',
            accessorFn: row => getManagerLabel(row.manager, actions.managerLabels),
            enableSorting: true,
            sortingFn: 'text',
            header: t('table.manager'),
            cell: ({ row }) =>
                h(
                    Badge,
                    {
                        variant: 'outline',
                        class: 'manager-badge rounded-sm',
                        style: { '--manager-hue': String(getManagerHue(row.original.manager)) },
                    },
                    () => getManagerLabel(row.original.manager, actions.managerLabels)
                ),
        },
        {
            id: 'actions',
            enableSorting: false,
            header: () => h('div', { class: 'text-right' }, t('table.actions')),
            cell: ({ row }) => {
                const pkg = row.original
                const upgradeKey = `upgrade:${pkg.manager}:${pkg.name}`
                const uninstallKey = `uninstall:${pkg.manager}:${pkg.name}`

                if (actions.mode === 'search') {
                    return h('div', { class: 'flex justify-end' }, [
                        h(VersionSelectButton, {
                            package: pkg,
                            disabled: actions.loadingKey !== null,
                            loadingKey: actions.loadingKey,
                            onInstall: actions.onInstall!,
                        }),
                    ])
                }

                return h(
                    'div',
                    { class: 'flex justify-end gap-2' },
                    [
                        actionTooltip(
                            actions.loadingKey === upgradeKey
                                ? t('actions.updating')
                                : t('actions.update'),
                            h(
                                Button,
                                {
                                    variant: 'ghost',
                                    size: 'icon',
                                    'aria-label':
                                        actions.loadingKey === upgradeKey
                                            ? t('actions.updating')
                                            : t('actions.update'),
                                    disabled:
                                        !row.original.outdated || actions.loadingKey !== null,
                                    onClick: () => actions.onUpgrade?.(pkg),
                                },
                                () => [h(ArrowUpCircle, { class: 'size-4' })]
                            )
                        ),
                        actionTooltip(
                            actions.loadingKey === uninstallKey
                                ? t('actions.removing')
                                : t('actions.remove'),
                            h(
                                Button,
                                {
                                    variant: 'ghost',
                                    size: 'icon',
                                    'aria-label':
                                        actions.loadingKey === uninstallKey
                                            ? t('actions.removing')
                                            : t('actions.remove'),
                                    class: 'text-[hsl(var(--destructive))] hover:text-[hsl(var(--destructive))]',
                                    disabled: actions.loadingKey !== null,
                                    onClick: () => actions.onUninstall?.(pkg),
                                },
                                () => [h(Trash2, { class: 'size-4' })]
                            )
                        ),
                    ].filter(Boolean)
                )
            },
        }
    )

    return columns
}

function getManagerLabel(manager: string, managerLabels?: Record<string, string>) {
    return managerLabels?.[manager] ?? managerLabelMap[manager] ?? manager
}

function actionTooltip(label: string, trigger: ReturnType<typeof h>) {
    return h(Tooltip, null, {
        default: () => [
            h(TooltipTrigger, null, {
                default: () => trigger,
            }),
            h(TooltipContent, null, {
                default: () => label,
            }),
        ],
    })
}
