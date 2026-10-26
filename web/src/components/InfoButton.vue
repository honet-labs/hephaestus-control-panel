<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue';
import { Info, X } from 'lucide-vue-next';

defineProps<{
  title: string;
  description: string;
}>();

const isOpen = ref(false);

const handleKeyDown = (e: KeyboardEvent) => {
  if (e.key === 'Escape' && isOpen.value) {
    isOpen.value = false;
  }
};

onMounted(() => {
  window.addEventListener('keydown', handleKeyDown);
});

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeyDown);
});
</script>

<template>
  <div class="inline-flex items-center">
    <button
      type="button"
      @click="isOpen = true"
      class="text-slate-400 hover:text-slate-600 dark:text-slate-500 dark:hover:text-slate-300 p-0.5 rounded transition cursor-pointer"
      :title="title || 'Information'"
      aria-label="Information Guide"
    >
      <Info class="w-4 h-4" />
    </button>

    <!-- Information Modal Popup -->
    <Teleport to="body">
      <div
        v-if="isOpen"
        class="fixed inset-0 z-[100] flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
        @click.self="isOpen = false"
      >
        <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-sm shadow-2xl p-5 space-y-4 font-sans">
          <div class="flex items-start justify-between gap-3 border-b border-slate-100 dark:border-slate-800 pb-3">
            <div class="flex items-center gap-2.5">
              <div class="w-8 h-8 rounded-lg bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 flex items-center justify-center shrink-0">
                <Info class="w-4 h-4" />
              </div>
              <div>
                <h3 class="text-sm font-bold text-slate-900 dark:text-white leading-tight">
                  {{ title }}
                </h3>
                <span class="text-[10px] text-slate-400 font-medium">Information Guide</span>
              </div>
            </div>
            <button
              type="button"
              @click="isOpen = false"
              class="text-slate-400 hover:text-slate-700 dark:hover:text-white p-1 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 transition cursor-pointer"
            >
              <X class="w-4 h-4" />
            </button>
          </div>

          <div class="text-xs text-slate-600 dark:text-slate-300 leading-relaxed whitespace-pre-line">
            {{ description }}
          </div>

          <div class="flex justify-end pt-2 border-t border-slate-100 dark:border-slate-800">
            <button
              type="button"
              @click="isOpen = false"
              class="px-4 py-1.5 bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 text-slate-800 dark:text-slate-200 rounded-lg text-xs font-semibold transition cursor-pointer"
            >
              Close
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
