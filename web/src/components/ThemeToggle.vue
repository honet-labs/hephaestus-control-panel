<script setup lang="ts">
import { computed } from 'vue';
import { useThemeStore } from '../stores/theme';
import { Sun, Moon } from 'lucide-vue-next';

const props = withDefaults(
  defineProps<{
    variant?: 'button' | 'pill' | 'compact';
    showLabel?: boolean;
  }>(),
  {
    variant: 'button',
    showLabel: false,
  }
);

const themeStore = useThemeStore();
const isDark = computed(() => themeStore.isDark);
</script>

<template>
  <!-- Variant 1: Elegant Icon Button -->
  <button
    v-if="variant === 'button'"
    @click="themeStore.toggleTheme"
    type="button"
    :title="isDark ? 'Switch to Light Mode' : 'Switch to Dark Mode'"
    class="flex items-center gap-2 p-1.5 rounded-lg border transition duration-200 focus:outline-none"
    :class="[
      isDark
        ? 'bg-[#1b2234] hover:bg-slate-700/60 border-slate-700/70 text-[#95CCDD] hover:text-white'
        : 'bg-[#eef3f9] hover:bg-[#dfe9f4] border-[#cfddec] text-[#293681] hover:text-[#4274D9] shadow-sm'
    ]"
  >
    <Sun v-if="isDark" class="w-4 h-4 text-amber-400 animate-in spin-in-180 duration-200" />
    <Moon v-else class="w-4 h-4 text-[#293681] animate-in spin-in-180 duration-200" />
    <span v-if="showLabel" class="text-xs font-medium font-sans">
      {{ isDark ? 'Light Mode' : 'Dark Mode' }}
    </span>
  </button>

  <!-- Variant 2: Compact Icon Only -->
  <button
    v-else-if="variant === 'compact'"
    @click="themeStore.toggleTheme"
    type="button"
    :title="isDark ? 'Switch to Light Mode' : 'Switch to Dark Mode'"
    class="p-1.5 rounded-lg transition duration-200 focus:outline-none"
    :class="[
      isDark
        ? 'text-slate-400 hover:text-amber-300 hover:bg-slate-800'
        : 'text-slate-600 hover:text-[#293681] hover:bg-[#e4edf6]'
    ]"
  >
    <Sun v-if="isDark" class="w-4 h-4 text-amber-400" />
    <Moon v-else class="w-4 h-4 text-[#293681]" />
  </button>

  <!-- Variant 3: Sliding Pill Switch -->
  <div
    v-else
    @click="themeStore.toggleTheme"
    :title="isDark ? 'Switch to Light Mode' : 'Switch to Dark Mode'"
    class="relative inline-flex h-6 w-12 items-center rounded-full transition-colors duration-300 cursor-pointer select-none border"
    :class="[
      isDark
        ? 'bg-[#1b2234] border-slate-700'
        : 'bg-[#D0E7E6] border-[#95CCDD]'
    ]"
  >
    <span
      class="inline-block h-4 w-4 transform rounded-full transition-transform duration-300 shadow-md flex items-center justify-center"
      :class="[
        isDark
          ? 'translate-x-7 bg-[#4274D9] text-white'
          : 'translate-x-1 bg-[#293681] text-[#D0E7E6]'
      ]"
    >
      <Moon v-if="isDark" class="w-2.5 h-2.5 text-white" />
      <Sun v-else class="w-2.5 h-2.5 text-amber-300" />
    </span>
  </div>
</template>
