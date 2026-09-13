<script setup lang="ts">
import { ref } from 'vue';
import { useRouter } from 'vue-router';
import {
  Server,
  Cpu,
  HardDrive,
  Layers,
  Sparkles,
  Terminal,
  Link2,
  ExternalLink,
  ShieldCheck,
} from 'lucide-vue-next';

const router = useRouter();

const plannedFeatures = [
  {
    icon: Server,
    title: 'Hypervisor Integration',
    desc: 'Native connectors for Proxmox VE, VMware ESXi, and KVM/QEMU hypervisor nodes with secure API authentication.',
    tag: 'Planned',
  },
  {
    icon: Cpu,
    title: 'Automated VM Discovery',
    desc: 'Real-time detection of VM instances, vCPU core allocations, dynamic RAM usage, and virtual storage volumes.',
    tag: 'Planned',
  },
  {
    icon: Layers,
    title: 'Clustering & Bare Metal',
    desc: 'Hardware telemetry, network bridge mapping, storage pool capacity, and unified cluster health metrics.',
    tag: 'Planned',
  },
];
</script>

<template>
  <div class="space-y-6 max-w-7xl mx-auto font-sans">
    <!-- Header -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-slate-200 dark:border-[#1b2234] pb-4">
      <div>
        <div class="flex items-center gap-2.5">
          <h1 class="text-xl font-bold text-slate-900 dark:text-white tracking-tight">Inventory Server</h1>
          <span class="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-[11px] font-mono font-semibold bg-blue-50 text-blue-700 border border-blue-200 dark:bg-blue-500/10 dark:text-[#95CCDD] dark:border-[#4274D9]/30">
            <Sparkles class="w-3 h-3 text-blue-600 dark:text-[#95CCDD]" />
            Upcoming Feature
          </span>
        </div>
        <p class="text-xs text-slate-600 dark:text-slate-400 mt-1">
          Virtual machine instances, hypervisors, and infrastructure node inventory management.
        </p>
      </div>

      <!-- Quick Action Buttons -->
      <div class="flex items-center gap-2">
        <router-link
          to="/connections"
          class="inline-flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-semibold text-slate-700 dark:text-slate-200 bg-slate-100 dark:bg-[#141824] hover:bg-slate-200 dark:hover:bg-[#1a2336] border border-slate-300 dark:border-[#222c42] transition"
        >
          <Link2 class="w-3.5 h-3.5 text-blue-600 dark:text-[#95CCDD]" />
          <span>Connections</span>
        </router-link>

        <a
          href="/remote-server"
          target="_blank"
          class="inline-flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-semibold text-white bg-blue-600 hover:bg-blue-700 dark:bg-[#293681] dark:hover:bg-[#3446a8] border border-blue-500/30 transition shadow-sm"
        >
          <Terminal class="w-3.5 h-3.5" />
          <span>Remote Server</span>
          <ExternalLink class="w-3 h-3 opacity-70" />
        </a>
      </div>
    </div>

    <!-- Empty State Hero Card -->
    <div class="p-8 sm:p-12 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-2xl shadow-sm text-center">
      <div class="max-w-md mx-auto space-y-4">
        <!-- Center Icon -->
        <div class="w-16 h-16 mx-auto rounded-2xl bg-blue-50 dark:bg-[#141d30] border border-blue-200 dark:border-[#293681] flex items-center justify-center text-blue-600 dark:text-[#95CCDD] shadow-inner">
          <Server class="w-8 h-8" />
        </div>

        <div>
          <h2 class="text-base font-bold text-slate-900 dark:text-white">VM Inventory Empty</h2>
          <p class="text-xs text-slate-600 dark:text-slate-400 mt-2 leading-relaxed">
            Halaman VM & Server Inventory sedang dikosongkan untuk persiapan fitur baru. Integrasi hypervisor, monitoring resource instance, dan manajemen cluster virtualisasi akan segera hadir di sini.
          </p>
        </div>

        <div class="pt-2">
          <div class="inline-flex items-center gap-2 px-3.5 py-1.5 rounded-full text-[11px] font-mono bg-slate-100 dark:bg-[#141824] border border-slate-200 dark:border-[#1b2234] text-slate-600 dark:text-slate-400">
            <span class="w-2 h-2 rounded-full bg-amber-500 animate-pulse"></span>
            <span>Module In Development</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Planned Capabilities Grid -->
    <div>
      <div class="mb-3">
        <h3 class="text-xs font-bold uppercase tracking-wider text-slate-500 dark:text-[#95CCDD]/80">
          Planned Feature Roadmap
        </h3>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div
          v-for="(feature, idx) in plannedFeatures"
          :key="idx"
          class="p-5 bg-white dark:bg-[#0e121c] border border-slate-200 dark:border-[#1b2234] rounded-xl space-y-3 hover:border-blue-300 dark:hover:border-[#293681] transition"
        >
          <div class="flex items-center justify-between">
            <div class="w-8 h-8 rounded-lg bg-blue-50 dark:bg-[#141d30] border border-blue-200 dark:border-[#293681]/50 flex items-center justify-center text-blue-600 dark:text-[#95CCDD]">
              <component :is="feature.icon" class="w-4 h-4" />
            </div>
            <span class="text-[10px] font-mono font-bold uppercase px-2 py-0.5 rounded bg-slate-100 dark:bg-[#141824] text-slate-600 dark:text-slate-400 border border-slate-200 dark:border-[#1b2234]">
              {{ feature.tag }}
            </span>
          </div>

          <div>
            <h4 class="text-xs font-bold text-slate-900 dark:text-white">{{ feature.title }}</h4>
            <p class="text-[11px] text-slate-600 dark:text-slate-400 mt-1 leading-relaxed">
              {{ feature.desc }}
            </p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
