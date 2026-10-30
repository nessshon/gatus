<template>
  <!-- Mirrors EndpointCard's boxes so the grid doesn't move when the real card replaces it -->
  <Card class="endpoint h-full flex flex-col" aria-hidden="true">
    <CardHeader class="endpoint-header px-3 sm:px-6 pt-3 sm:pt-6 pb-2 space-y-0">
      <div class="flex items-start justify-between gap-2 sm:gap-3">
        <div class="flex-1 min-w-0 overflow-hidden">
          <CardTitle class="text-base sm:text-lg">
            <span class="block h-5 sm:h-6 py-0.5"><span :class="['block h-full w-2/3', bone]"></span></span>
          </CardTitle>
          <div class="flex items-center min-h-[1.25rem]">
            <span :class="['h-3 w-1/3', bone]"></span>
          </div>
        </div>
        <div class="flex-shrink-0 ml-2">
          <span :class="['block h-[22px] w-[72px] rounded-full', bone]"></span>
        </div>
      </div>
    </CardHeader>
    <CardContent class="endpoint-content flex-1 pb-3 sm:pb-4 px-3 sm:px-6 pt-2">
      <div class="space-y-2">
        <div>
          <div class="flex items-center justify-end mb-1 h-4">
            <span :class="['h-3 w-10', bone]"></span>
          </div>
          <div class="flex gap-0.5">
            <div
              v-for="index in maxResults"
              :key="index"
              class="flex-1 h-6 sm:h-8 rounded-sm bg-gray-200 dark:bg-gray-700 animate-pulse motion-reduce:animate-none"
            />
          </div>
          <div class="flex items-center justify-between mt-1 h-4">
            <span :class="['h-3 w-12', bone]"></span>
            <span :class="['h-3 w-8', bone]"></span>
          </div>
        </div>
      </div>
    </CardContent>
  </Card>
</template>

<script setup>
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card'

defineProps({
  maxResults: {
    type: Number,
    default: 50
  }
})

const bone = 'rounded bg-muted animate-pulse motion-reduce:animate-none'
</script>
