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
  <div class="h-full flex flex-col space-y-4">
    <div>
      <h2 class="text-xl font-bold text-white tracking-tight">Grok Regex Debugger</h2>
      <p class="text-xs text-slate-400">Test and validate log parsing Grok patterns interactively</p>
    </div>

    <div class="grid grid-cols-1 lg:grid-cols-2 gap-4 flex-1 min-h-0">
      <!-- Input Panel -->
      <div class="p-4 bg-slate-900/60 border border-slate-800 rounded-xl space-y-4 flex flex-col">
        <div class="space-y-1">
          <label class="block text-xs font-semibold text-slate-400">Grok Pattern</label>
          <textarea
            v-model="pattern"
            rows="3"
            class="w-full bg-slate-800 border border-slate-700 rounded-lg p-2.5 text-xs text-brand-400 font-mono focus:outline-none focus:border-brand-500"
            placeholder="%{TIMESTAMP:timestamp} %{WORD:level}..."
          ></textarea>
        </div>

        <div class="space-y-1 flex-1 flex flex-col">
          <label class="block text-xs font-semibold text-slate-400">Sample Log Line</label>
          <textarea
            v-model="sampleText"
            class="w-full flex-1 bg-slate-800 border border-slate-700 rounded-lg p-2.5 text-xs text-slate-200 font-mono focus:outline-none"
            placeholder="Paste log line here..."
          ></textarea>
        </div>

        <button
          @click="testPattern"
          :disabled="loading"
          class="w-full py-2 bg-brand-500 hover:bg-brand-600 text-white font-medium text-xs rounded-lg transition flex items-center justify-center gap-1.5"
        >
          <Play class="w-3.5 h-3.5 fill-current" />
          Test Pattern
        </button>
      </div>

      <!-- Result Panel -->
      <div class="p-4 bg-slate-900/60 border border-slate-800 rounded-xl space-y-3 flex flex-col min-w-0">
        <div class="flex items-center justify-between">
          <h3 class="text-xs font-bold text-white uppercase tracking-wider">Extraction Matches</h3>
          <span v-if="result" :class="[
            result.matched ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20' : 'bg-red-500/10 text-red-400 border-red-500/20',
            'px-2 py-0.5 rounded text-[10px] font-bold border'
          ]">
            {{ result.matched ? 'MATCHED' : 'NO MATCH' }}
          </span>
        </div>

        <div class="flex-1 bg-slate-950 border border-slate-800/80 rounded-lg p-3 font-mono text-[11px] overflow-y-auto">
          <pre v-if="result" class="text-slate-300">{{ JSON.stringify(result.matches, null, 2) }}</pre>
          <div v-else class="h-full flex items-center justify-center text-slate-600 text-xs font-sans">
            Press 'Test Pattern' to run extraction
          </div>
        </div>

        <div v-if="result?.regex" class="text-[10px] text-slate-500 font-mono truncate">
          Regex: {{ result.regex }}
        </div>
      </div>
    </div>
  </div>
</template>
