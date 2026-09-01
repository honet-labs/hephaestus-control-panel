import { defineStore } from 'pinia';
import { ref } from 'vue';
import axios from 'axios';

export interface User {
  id: number;
  username: string;
  role: string;
  forcePasswordChange: boolean;
}

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null);
  const token = ref<string | null>(localStorage.getItem('hephaestus_token'));
  const isAuthenticated = ref<boolean>(!!token.value);

  // Attach token to axios defaults
  if (token.value) {
    axios.defaults.headers.common['Authorization'] = `Bearer ${token.value}`;
  }

  const setAuth = (newUser: User, newToken: string) => {
    user.value = newUser;
    token.value = newToken;
    isAuthenticated.value = true;
    localStorage.setItem('hephaestus_token', newToken);
    axios.defaults.headers.common['Authorization'] = `Bearer ${newToken}`;
  };

  const clearAuth = () => {
    user.value = null;
    token.value = null;
    isAuthenticated.value = false;
    localStorage.removeItem('hephaestus_token');
    delete axios.defaults.headers.common['Authorization'];
  };

  const fetchUser = async () => {
    if (!token.value) return null;
    try {
      const res = await axios.get('/api/v1/auth/me');
      if (res.data.success) {
        user.value = res.data.data;
        return user.value;
      }
    } catch {
      clearAuth();
    }
    return null;
  };

  const logout = async () => {
    try {
      await axios.post('/api/v1/auth/logout');
    } catch (_) {}
    clearAuth();
  };

  return {
    user,
    token,
    isAuthenticated,
    setAuth,
    clearAuth,
    fetchUser,
    logout,
  };
});
