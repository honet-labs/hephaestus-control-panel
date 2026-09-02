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
        <div class="w-14 h-14 rounded-2xl bg-gradient-to-tr from-[#293681] to-[#4274D9] mx-auto flex items-center justify-center shadow-xl shadow-[#4274D9]/30 font-mono font-black text-sm text-white tracking-tight">
          HCP
        </div>
        <h1 class="text-xl font-bold text-white tracking-tight">HEPHAESTUS</h1>
        <p class="text-xs text-[#95CCDD] font-medium">Control Panel (HCP) — Sign In</p>
      </div>

      <!-- Login Form Card -->
      <div class="p-6 bg-[#0e121c] border border-[#1b2234] rounded-2xl shadow-2xl backdrop-blur space-y-4">
        <div v-if="error" class="p-3 rounded-lg bg-rose-500/10 border border-rose-500/20 text-rose-400 text-xs flex items-center gap-2">
          <AlertCircle class="w-4 h-4 shrink-0" />
          <span>{{ error }}</span>
        </div>

        <form @submit.prevent="handleLogin" class="space-y-4 text-xs">
          <div>
            <label class="block text-slate-400 mb-1 font-medium">Username</label>
            <div class="relative">
              <User class="w-4 h-4 absolute left-3 top-2.5 text-[#95CCDD]" />
              <input
                v-model="username"
                type="text"
                required
                class="w-full bg-[#141824] border border-[#1b2234] rounded-lg pl-9 pr-3 py-2 text-white placeholder-slate-500 focus:outline-none focus:border-[#4274D9] transition"
                placeholder="admin"
              />
            </div>
          </div>

          <div>
            <label class="block text-slate-400 mb-1 font-medium">Password</label>
            <div class="relative">
              <Lock class="w-4 h-4 absolute left-3 top-2.5 text-[#95CCDD]" />
              <input
                v-model="password"
                type="password"
                required
                class="w-full bg-[#141824] border border-[#1b2234] rounded-lg pl-9 pr-3 py-2 text-white placeholder-slate-500 focus:outline-none focus:border-[#4274D9] transition"
                placeholder="••••••••"
              />
            </div>
          </div>

          <button
            type="submit"
            :disabled="loading"
            class="w-full py-2.5 bg-[#4274D9] hover:bg-[#3461c2] disabled:opacity-50 text-white font-bold rounded-lg shadow-lg shadow-[#4274D9]/25 transition duration-150"
          >
            {{ loading ? 'Signing In...' : 'Sign In' }}
          </button>
        </form>
      </div>

      <div class="text-center">
        <p class="text-[11px] text-[#95CCDD]/70 flex items-center justify-center gap-1.5 font-medium">
          <ShieldCheck class="w-3.5 h-3.5 text-[#4274D9]" />
          Protected by HCP End-to-End Vault Authentication
        </p>
      </div>
    </div>
  </div>
</template>
