<script setup lang="ts">
import { ref } from 'vue';
import axios from 'axios';
import { ListTree, Play, CheckCircle2, XCircle } from 'lucide-vue-next';

const pattern = ref('%{TIMESTAMP:timestamp} %{WORD:level} \\[%{WORD:module}\\] %{NOTSPACE:message}');
const sampleText = ref('2026-09-01T22:00:00Z INFO [SSH] Terminal session connected successfully');
const result = ref<any>(null);
const loading = ref(false);

const testPattern = async () => {
  loading.value = true;
  try {
    const res = await axios.post('/api/v1/grok/test', {
      pattern: pattern.value,
      text: sampleText.value,
    });
    if (res.data.success) {
      result.value = res.data.data;
    }
  } catch (err: any) {
    alert(`Failed to test pattern: ${err.response?.data?.error || err.message}`);
  } finally {
    loading.value = false;
  }
};
</script>

<template>
  <div class="space-y-6 max-w-7xl mx-auto font-sans">
    <!-- Header -->
    <div class="border-b border-slate-200 dark:border-[#1b2234] pb-4">
      <h1 class="text-xl font-bold text-slate-900 dark:text-white tracking-tight">Grok Regex Debugger</h1>
      <p class="text-xs text-blue-700 dark:text-[#95CCDD]/80 mt-0.5">
        Test and validate log parsing Grok patterns interactively against sample log lines.
      </p>
    </div>

    <div class="grid grid-cols-1 lg:grid-cols-12 gap-6">
      <!-- Input Panel (Left) -->
      <div class="lg:col-span-6 p-5 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl space-y-4 shadow-sm flex flex-col">
        <div class="flex items-center justify-between border-b border-slate-200 dark:border-[#1b2234] pb-3">
          <h2 class="text-xs font-bold text-slate-900 dark:text-white uppercase tracking-wider flex items-center gap-1.5">
            <ListTree class="w-3.5 h-3.5 text-blue-600 dark:text-[#95CCDD]" />
            <span>Pattern Configuration</span>
          </h2>
        </div>

        <div class="space-y-1.5">
          <label class="block text-xs font-bold text-slate-700 dark:text-[#D0E7E6] uppercase tracking-wider">Grok Pattern</label>
          <textarea
            v-model="pattern"
            rows="3"
            class="w-full bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg p-3 text-xs text-blue-700 dark:text-[#95CCDD] font-mono font-semibold placeholder-slate-400 dark:placeholder-slate-600 focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500/20"
            placeholder="%{TIMESTAMP:timestamp} %{WORD:level}..."
          ></textarea>
        </div>

        <div class="space-y-1.5 flex-1 flex flex-col">
          <label class="block text-xs font-bold text-slate-700 dark:text-[#D0E7E6] uppercase tracking-wider">Sample Log Line</label>
          <textarea
            v-model="sampleText"
            rows="6"
            class="w-full flex-1 bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg p-3 text-xs text-slate-900 dark:text-white font-mono placeholder-slate-400 dark:placeholder-slate-600 focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500/20"
            placeholder="Paste log line here..."
          ></textarea>
        </div>

        <button
          @click="testPattern"
          :disabled="loading"
          class="w-full py-2.5 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white font-semibold text-xs rounded-lg transition flex items-center justify-center gap-1.5 shadow-sm"
        >
          <Play class="w-3.5 h-3.5 fill-current" />
          <span>{{ loading ? 'Testing Pattern...' : 'Test Pattern' }}</span>
        </button>
      </div>

      <!-- Result Panel (Right) -->
      <div class="lg:col-span-6 p-5 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl space-y-4 shadow-sm flex flex-col min-w-0">
        <div class="flex items-center justify-between border-b border-slate-200 dark:border-[#1b2234] pb-3">
          <h2 class="text-xs font-bold text-slate-900 dark:text-white uppercase tracking-wider">Extraction Matches</h2>
          <span
            v-if="result"
            :class="[
              result.matched
                ? 'bg-emerald-50 dark:bg-emerald-500/10 text-emerald-700 dark:text-emerald-400 border-emerald-300 dark:border-emerald-500/30'
                : 'bg-rose-50 dark:bg-rose-500/10 text-rose-700 dark:text-rose-400 border-rose-300 dark:border-rose-500/30',
              'px-2.5 py-0.5 rounded-full text-[10px] font-bold border font-mono'
            ]"
          >
            {{ result.matched ? 'MATCHED' : 'NO MATCH' }}
          </span>
        </div>

        <div class="flex-1 bg-slate-50 dark:bg-[#121826] border border-slate-200 dark:border-[#1b2234] rounded-lg p-3.5 font-mono text-[11px] overflow-y-auto min-h-[300px]">
          <pre v-if="result && result.matched" class="text-slate-900 dark:text-slate-200 leading-relaxed font-semibold">{{ JSON.stringify(result.matches, null, 2) }}</pre>
          <div v-else-if="result && !result.matched" class="h-full flex flex-col items-center justify-center text-rose-600 dark:text-rose-400 text-xs font-sans gap-1">
            <XCircle class="w-6 h-6" />
            <span class="font-bold">Pattern did not match sample log line</span>
          </div>
          <div v-else class="h-full flex flex-col items-center justify-center text-slate-500 dark:text-slate-400 text-xs font-sans gap-2">
            <ListTree class="w-8 h-8 text-slate-300 dark:text-slate-600" />
            <span>Click 'Test Pattern' to run extraction</span>
          </div>
        </div>

        <div v-if="result?.regex" class="text-[11px] text-slate-600 dark:text-slate-400 font-mono truncate bg-slate-100 dark:bg-[#171b26] px-3 py-1.5 rounded border border-slate-200 dark:border-[#1b2234]">
          <span class="font-bold text-slate-800 dark:text-slate-200">Compiled Regex:</span> {{ result.regex }}
        </div>
      </div>
    </div>
  </div>
</template>
