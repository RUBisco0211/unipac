<script setup lang="ts">
import { TooltipArrow, TooltipContent, TooltipPortal } from 'reka-ui'
</script>

<template>
    <TooltipPortal>
        <TooltipContent
            :side-offset="8"
            class="z-50 overflow-hidden rounded-md border border-[hsl(var(--border))] bg-[hsl(var(--popover))] px-3 py-1.5 text-xs text-[hsl(var(--popover-foreground))] shadow-md data-[state=delayed-open]:animate-in data-[state=closed]:animate-out"
        >
            <slot />
            <TooltipArrow class="fill-[hsl(var(--popover))]" />
        </TooltipContent>
    </TooltipPortal>
</template>
