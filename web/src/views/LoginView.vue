<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import axios from 'axios';
import { useAuthStore } from '../stores/auth';
import { ShieldCheck, Lock, User, AlertCircle } from 'lucide-vue-next';

const username = ref('');
const password = ref('');
const error = ref('');
const loading = ref(false);
const router = useRouter();
const authStore = useAuthStore();

const checkSetup = async () => {
  try {
    const res = await axios.get('/api/v1/setup/status');
    if (res.data.success && !res.data.data.setupCompleted) {
      router.push('/setup');
    }
  } catch (_) {}
};

const handleLogin = async () => {
  error.value = '';
  loading.value = true;
  try {
    const res = await axios.post('/api/v1/auth/login', {
      username: username.value,
      password: password.value,
    });
    if (res.data.success) {
      authStore.setAuth(res.data.data.user, res.data.data.token);
      router.push('/');
    }
  } catch (err: any) {
    error.value = err.response?.data?.message || 'Login failed. Please check credentials.';
  } finally {
    loading.value = false;
  }
};

onMounted(() => {
  checkSetup();
});
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-[#090d16] p-4 font-sans">
    <div class="w-full max-w-sm space-y-6">
      <!-- Logo Brand -->
      <div class="text-center space-y-2">
        <div class="w-12 h-12 rounded-xl bg-gradient-to-tr from-brand-600 to-emerald-400 mx-auto flex items-center justify-center shadow-xl shadow-brand-500/20">
          <ShieldCheck class="w-7 h-7 text-white" />
        </div>
        <h1 class="text-xl font-bold text-white tracking-tight">HEPHAESTUS</h1>
        <p class="text-xs text-slate-400">Sign in to your DevOps control plane</p>
      </div>

      <!-- Login Form Card -->
      <div class="p-6 bg-slate-900/60 border border-slate-800 rounded-2xl shadow-2xl backdrop-blur space-y-4">
        <div v-if="error" class="p-3 rounded-lg bg-red-500/10 border border-red-500/20 text-red-400 text-xs flex items-center gap-2">
          <AlertCircle class="w-4 h-4 shrink-0" />
          <span>{{ error }}</span>
        </div>

        <form @submit.prevent="handleLogin" class="space-y-4 text-xs">
          <div>
            <label class="block text-slate-400 mb-1 font-medium">Username</label>
            <div class="relative">
              <User class="w-4 h-4 absolute left-3 top-2.5 text-slate-500" />
              <input
                v-model="username"
                type="text"
                required
                class="w-full bg-slate-800 border border-slate-700 rounded-lg pl-9 pr-3 py-2 text-white placeholder-slate-500 focus:outline-none focus:border-brand-500 transition"
                placeholder="admin"
              />
            </div>
          </div>

          <div>
            <label class="block text-slate-400 mb-1 font-medium">Password</label>
            <div class="relative">
              <Lock class="w-4 h-4 absolute left-3 top-2.5 text-slate-500" />
              <input
                v-model="password"
                type="password"
                required
                class="w-full bg-slate-800 border border-slate-700 rounded-lg pl-9 pr-3 py-2 text-white placeholder-slate-500 focus:outline-none focus:border-brand-500 transition"
                placeholder="••••••••"
              />
            </div>
          </div>

          <button
            type="submit"
            :disabled="loading"
            class="w-full py-2.5 bg-brand-500 hover:bg-brand-600 disabled:opacity-50 text-white font-semibold rounded-lg shadow-lg shadow-brand-500/20 transition duration-150"
          >
            {{ loading ? 'Signing In...' : 'Sign In' }}
          </button>
        </form>
      </div>
    </div>
  </div>
</template>
