<script setup lang="ts">
defineProps<{
  tone: 'success' | 'error'
  message: string
}>()
</script>

<template>
  <Message :severity="tone" :closable="false">{{ message }}</Message>
</template>
