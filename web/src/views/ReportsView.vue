<script setup lang="ts">
import { ref } from 'vue';
import { useRouter } from 'vue-router';
import {
  FileText,
  Layers,
  Sparkles,
  ArrowRight,
  Clock,
  CheckCircle2,
  BarChart3,
  Printer,
  ShieldCheck,
} from 'lucide-vue-next';

const router = useRouter();

const upcomingHighlights = ref([
  {
    title: 'Printable Multi-Page A4 Canvas',
    desc: 'Pixel-perfect document layout designer with custom header branding, page numbers, and orientation.',
  },
  {
    title: 'Modular Visual Widget Palette',
    desc: 'Drag and arrange telemetry charts, incident summaries, and executive status tables across pages.',
  },
  {
    title: 'Automated Scheduled PDF Delivery',
    desc: 'Background cron generator delivering periodic executive PDF summaries directly via email.',
  },
  {
    title: 'Cross-Cluster Data Consolidation',
    desc: 'Combine OpenSearch log aggregations, Prometheus host telemetry, and Grafana panels in a single report.',
  },
]);
</script>

<template>
  <div class="space-y-6 max-w-7xl mx-auto font-sans text-slate-800 dark:text-slate-200">
    <!-- Header (Strictly complies with AGENTS.md: No icon in H1) -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-slate-200 dark:border-[#1b2234] pb-4">
      <div>
        <h1 class="text-xl font-bold text-slate-900 dark:text-white tracking-tight">
          Visual Report
        </h1>
        <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
          Custom visual document canvas and multi-page printable executive reporting.
        </p>
      </div>
      <div class="flex items-center gap-2 shrink-0">
        <router-link
          to="/reports/raw"
          class="px-3.5 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-semibold shadow-xs transition flex items-center gap-1.5 cursor-pointer"
        >
          <span>Open Raw Data Report</span>
          <ArrowRight class="w-3.5 h-3.5" />
        </router-link>
      </div>
    </div>

    <!-- Coming Soon Banner Card -->
    <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl p-8 sm:p-12 text-center max-w-3xl mx-auto space-y-6 shadow-sm">
      <div class="w-16 h-16 rounded-2xl bg-blue-50 dark:bg-blue-950/50 border border-blue-200 dark:border-blue-900/60 text-blue-600 dark:text-[#95CCDD] flex items-center justify-center mx-auto shadow-xs">
        <Clock class="w-8 h-8 text-slate-600 dark:text-slate-300" />
      </div>

      <div class="space-y-2 max-w-xl mx-auto">
        <div class="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-[11px] font-semibold bg-slate-100 dark:bg-[#1a2336] text-slate-700 dark:text-slate-300 border border-slate-200 dark:border-[#2a3650]">
          <Sparkles class="w-3.5 h-3.5 text-slate-500" />
          <span>Coming Soon Feature</span>
        </div>
        <h2 class="text-lg sm:text-xl font-bold text-slate-900 dark:text-white tracking-tight">
          Visual Document Canvas Report
        </h2>
        <p class="text-xs text-slate-500 dark:text-slate-400 leading-relaxed">
          Fitur pembuatan dokumen laporan visual multi-halaman berbasis canvas interaktif sedang dalam tahap penyempurnaan. Saat ini Anda dapat menggunakan modul <strong class="text-slate-700 dark:text-slate-300 font-semibold">Raw Data Report</strong> untuk memonitor data metrik Prometheus, Grafana, dan log OpenSearch dengan visualisasi Apache ECharts.
        </p>
      </div>

      <!-- Call To Action -->
      <div class="pt-2 flex flex-col sm:flex-row items-center justify-center gap-3">
        <button
          @click="router.push('/reports/raw')"
          class="w-full sm:w-auto px-5 py-2.5 bg-blue-600 hover:bg-blue-500 text-white rounded-xl text-xs font-semibold shadow-xs transition flex items-center justify-center gap-2 cursor-pointer"
        >
          <span>Buka Raw Data Report</span>
          <ArrowRight class="w-3.5 h-3.5" />
        </button>
      </div>

      <!-- Roadmap Feature Highlights -->
      <div class="pt-6 border-t border-slate-100 dark:border-[#1b2234] text-left">
        <p class="text-[11px] font-semibold text-slate-400 uppercase tracking-wider mb-4 text-center">
          Fitur yang Sedang Disiapkan
        </p>
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <div
            v-for="(item, idx) in upcomingHighlights"
            :key="idx"
            class="p-3.5 rounded-xl bg-slate-50 dark:bg-[#0c101a] border border-slate-200 dark:border-[#1f283d] space-y-1"
          >
            <div class="flex items-center gap-2 text-xs font-semibold text-slate-800 dark:text-slate-200">
              <CheckCircle2 class="w-3.5 h-3.5 text-slate-400 shrink-0" />
              <span>{{ item.title }}</span>
            </div>
            <p class="text-[11px] text-slate-500 dark:text-slate-400 pl-5.5 leading-normal">
              {{ item.desc }}
            </p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
