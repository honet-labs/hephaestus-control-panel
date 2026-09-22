<script setup lang="ts">
import { useRouter } from 'vue-router';
import { AlertCircle, ArrowLeft, Home } from 'lucide-vue-next';

const router = useRouter();

const goBack = () => {
  if (window.history.length > 1) {
    router.back();
  } else {
    router.push('/');
  }
};
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-slate-50 dark:bg-[#090d16] p-4 font-sans text-slate-800 dark:text-slate-100">
    <div class="w-full max-w-md bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl p-8 shadow-xl text-center space-y-6">
      <div class="w-16 h-16 rounded-2xl bg-slate-100 dark:bg-slate-800/80 text-slate-400 flex items-center justify-center mx-auto">
        <AlertCircle class="w-8 h-8" />
      </div>

      <div class="space-y-2">
        <span class="text-xs font-bold tracking-widest text-blue-600 dark:text-[#95CCDD] uppercase">Error 404</span>
        <h1 class="text-xl font-bold text-slate-900 dark:text-white tracking-tight">Halaman Tidak Ditemukan</h1>
        <p class="text-xs text-slate-500 dark:text-slate-400 leading-relaxed max-w-xs mx-auto">
          Alamat atau URL yang Anda tuju tidak tersedia atau telah dipindahkan.
        </p>
      </div>

      <div class="flex items-center justify-center gap-3 pt-2">
        <button
          @click="goBack"
          class="flex items-center gap-2 px-4 py-2 rounded-xl border border-slate-200 dark:border-slate-800 text-xs font-semibold text-slate-600 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-800 transition cursor-pointer"
        >
          <ArrowLeft class="w-3.5 h-3.5" />
          <span>Kembali</span>
        </button>

        <router-link
          to="/"
          class="flex items-center gap-2 px-4 py-2 rounded-xl bg-blue-600 hover:bg-blue-500 text-white text-xs font-bold transition cursor-pointer shadow-xs"
        >
          <Home class="w-3.5 h-3.5" />
          <span>Dashboard</span>
        </router-link>
      </div>
    </div>
  </div>
</template>
