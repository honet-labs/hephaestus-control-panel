<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { RouterLink } from 'vue-router';
import { ChevronLeft, ChevronRight, Flame, Radio, Search, Sliders } from 'lucide-vue-next';
import { useAuthStore } from '../stores/auth';
import { appServices, getServicePage } from '../lib/appServices';

const authStore = useAuthStore();
const services = computed(() => appServices.filter(service => authStore.can(service.feature, 'read')));
const page = ref(1);
const pageSize = ref(6);
const pageSizeOptions = [3, 6, 12, 24];
const pagination = computed(() => getServicePage(services.value, page.value, pageSize.value));
watch(pageSize, () => { page.value = 1; });
watch(() => pagination.value.page, value => { page.value = value; });
const icons = { opensearch: Search, dataprepper: Sliders, prometheus: Flame, opentelemetry: Radio };
</script>

<template>
  <div class="space-y-6 max-w-7xl mx-auto font-sans">
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-slate-200 dark:border-[#1b2234] pb-4">
      <div>
        <h1 class="text-xl font-bold text-slate-900 dark:text-white tracking-tight">App Services</h1>
        <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">Access application monitoring, pipelines, and telemetry configuration.</p>
      </div>
    </div>

    <div v-if="services.length" class="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-4" aria-label="Application services">
      <article v-for="service in pagination.items" :key="service.id" class="min-w-0 flex flex-col rounded-xl border border-slate-200 dark:border-[#293247] bg-white dark:bg-[#111624] shadow-sm transition hover:border-slate-400 dark:hover:border-slate-500 hover:shadow-md">
        <component
          :is="service.newTab ? 'a' : RouterLink"
          v-bind="service.newTab ? { href: service.path, target: '_blank', rel: 'noopener noreferrer' } : { to: service.path }"
          :aria-label="`${service.name}${service.newTab ? ' (opens in new tab)' : ''}`"
          class="group flex flex-1 items-start gap-3 p-4 sm:p-5 rounded-xl outline-none focus-visible:ring-2 focus-visible:ring-slate-500 focus-visible:ring-offset-2 dark:focus-visible:ring-offset-[#111624]"
        >
          <span class="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-[#182031] text-slate-500 dark:text-slate-400">
            <component :is="icons[service.id]" class="h-5 w-5" aria-hidden="true" />
          </span>
          <div class="min-w-0 flex-1 space-y-1.5">
            <h2 class="text-sm font-semibold text-slate-900 dark:text-slate-100">{{ service.name }}</h2>
            <p class="text-xs leading-relaxed text-slate-500 dark:text-slate-400">{{ service.description }}</p>
            <p class="text-xs font-medium text-slate-600 dark:text-slate-300 pt-1">{{ service.newTab ? 'Open monitoring' : 'Open configuration' }}</p>
          </div>
          <ChevronRight class="h-4 w-4 shrink-0 mt-0.5 text-slate-400 group-hover:text-slate-700 dark:group-hover:text-slate-200" aria-hidden="true" />
        </component>
        <RouterLink v-if="'visualPath' in service" :to="service.visualPath" class="mx-4 mb-4 sm:mx-5 sm:mb-5 w-fit text-xs font-medium text-slate-600 dark:text-slate-300 underline underline-offset-4 rounded focus-visible:outline focus-visible:outline-2 focus-visible:outline-slate-500">Open Visual Config</RouterLink>
      </article>
    </div>
    <div v-else class="rounded-xl border border-slate-200 dark:border-slate-700 p-6 text-sm text-slate-500 dark:text-slate-400">No application services are available with your current permissions.</div>

    <div role="navigation" aria-label="App Services pagination" class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 border-t border-slate-200 dark:border-[#293247] pt-4 text-xs text-slate-500 dark:text-slate-400">
      <p aria-live="polite">Showing {{ pagination.start }}–{{ pagination.end }} of {{ services.length }} services</p>
      <div class="flex flex-wrap items-center gap-3">
        <label class="flex items-center gap-2">
          <span>Per page</span>
          <select v-model.number="pageSize" aria-label="Services per page" class="rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-[#111624] text-slate-700 dark:text-slate-200 px-2 py-1.5 focus:outline-none focus:ring-2 focus:ring-slate-400">
            <option v-for="size in pageSizeOptions" :key="size" :value="size">{{ size }}</option>
          </select>
        </label>
        <div class="flex items-center gap-2">
          <button :disabled="pagination.page === 1" aria-label="Previous services page" @click="page = pagination.page - 1" class="p-2 rounded-lg border border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 disabled:opacity-40 disabled:cursor-not-allowed focus-visible:outline focus-visible:outline-2 focus-visible:outline-slate-500"><ChevronLeft class="w-4 h-4" aria-hidden="true" /></button>
          <span>Page {{ pagination.page }} of {{ pagination.totalPages }}</span>
          <button :disabled="pagination.page === pagination.totalPages" aria-label="Next services page" @click="page = pagination.page + 1" class="p-2 rounded-lg border border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 disabled:opacity-40 disabled:cursor-not-allowed focus-visible:outline focus-visible:outline-2 focus-visible:outline-slate-500"><ChevronRight class="w-4 h-4" aria-hidden="true" /></button>
        </div>
      </div>
    </div>
  </div>
</template>
