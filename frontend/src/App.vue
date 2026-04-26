<template>
  <RouterView />
  <AppToastHost />
  <AppDialogHost />
</template>

<script setup lang="ts">
import AppDialogHost from './components/AppDialogHost.vue'
import AppToastHost from './components/AppToastHost.vue'
</script>
