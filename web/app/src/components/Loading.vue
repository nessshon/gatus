<template>
  <div class="flex justify-center items-center">
    <img
      :class="[
        'animate-spin rounded-full opacity-60',
        sizeClass,
      ]"
      :src="logoSrc"
      alt=""
    />
  </div>
</template>

<script setup>
import { computed } from 'vue'
import bundledLogo from '../assets/logo.svg'

// Use the server-injected logo; the bundled one is the fallback (e.g. in dev, where the template is not rendered)
const logoSrc = computed(() => {
  const configured = window.config && window.config.logo
  return configured && configured !== '{{ .UI.Logo }}' ? configured : bundledLogo
})

const props = defineProps({
  size: {
    type: String,
    default: 'md',
    validator: (value) => ['xs', 'sm', 'md', 'lg', 'xl'].includes(value)
  },
})

const sizeClass = computed(() => {
  const sizes = {
    xs: 'w-4 h-4',
    sm: 'w-6 h-6',
    md: 'w-8 h-8',
    lg: 'w-12 h-12',
    xl: 'w-16 h-16'
  }
  return sizes[props.size] || sizes.md
})
</script>
