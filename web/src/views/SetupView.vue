<script setup lang="ts">
import { ref } from 'vue';
import { useRouter } from 'vue-router';
import axios from 'axios';
import { useAuthStore } from '../stores/auth';
import { ShieldCheck, Sparkles } from 'lucide-vue-next';

const username = ref('admin');
const password = ref('');
const confirmPassword = ref('');
const error = ref('');
const loading = ref(false);
const router = useRouter();
const authStore = useAuthStore();

const handleSetup = async () => {
  if (password.value !== confirmPassword.value) {
    error.value = 'Passwords do not match';
    return;
  }
  if (password.value.length < 6) {
    error.value = 'Password must be at least 6 characters';
    return;
  }

  error.value = '';
  loading.value = true;
  try {
    const res = await axios.post('/api/v1/setup/complete', {
      username: username.value,
      password: password.value,
    });
    if (res.data.success) {
      authStore.setAuth(res.data.data.user, res.data.data.token);
      router.push('/');
    }
  } catch (err: any) {
    error.value = err.response?.data?.error || 'Setup failed';
  } finally {
    loading.value = false;
  }
};
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-[#090d16] p-4 font-sans">
    <div class="w-full max-w-md space-y-6">
      <div class="text-center space-y-2">
        <div class="w-14 h-14 rounded-2xl bg-gradient-to-tr from-brand-600 to-emerald-400 mx-auto flex items-center justify-center shadow-xl shadow-brand-500/20 font-mono font-black text-sm text-white tracking-tight">
          HCP
        </div>
        <h1 class="text-xl font-bold text-white tracking-tight">Hephaestus Control Panel (HCP)</h1>
        <p class="text-xs text-slate-400">Initialize your master administrator account to get started</p>
      </div>

      <div class="p-6 bg-slate-900/60 border border-slate-800 rounded-2xl shadow-2xl backdrop-blur space-y-4">
        <div v-if="error" class="p-3 rounded-lg bg-red-500/10 border border-red-500/20 text-red-400 text-xs">
          {{ error }}
        </div>

        <form @submit.prevent="handleSetup" class="space-y-4 text-xs">
          <div>
            <label class="block text-slate-400 mb-1 font-medium">Administrator Username</label>
            <input v-model="username" required class="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-brand-500" />
          </div>

          <div>
            <label class="block text-slate-400 mb-1 font-medium">Password</label>
            <input v-model="password" type="password" required class="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-brand-500" placeholder="Minimum 6 characters" />
          </div>

          <div>
            <label class="block text-slate-400 mb-1 font-medium">Confirm Password</label>
            <input v-model="confirmPassword" type="password" required class="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-brand-500" placeholder="Re-enter password" />
          </div>

          <button
            type="submit"
            :disabled="loading"
            class="w-full py-2.5 bg-brand-500 hover:bg-brand-600 disabled:opacity-50 text-white font-semibold rounded-lg shadow-lg shadow-brand-500/20 transition"
          >
            {{ loading ? 'Initializing...' : 'Complete Installation' }}
          </button>
        </form>
      </div>
    </div>
  </div>
</template>
