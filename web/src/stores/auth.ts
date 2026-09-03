import { defineStore } from 'pinia';
import { ref } from 'vue';
import axios from 'axios';

export interface User {
  id: number;
  username: string;
  role: string;
  permissions?: Record<string, string>;
  forcePasswordChange: boolean;
}

export const useAuthStore = defineStore('auth', () => {
  const savedUserStr = localStorage.getItem('hephaestus_user');
  let initialUser: User | null = null;
  if (savedUserStr) {
    try {
      initialUser = JSON.parse(savedUserStr);
    } catch (_) {}
  }

  const user = ref<User | null>(initialUser);
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
    localStorage.setItem('hephaestus_user', JSON.stringify(newUser));
    axios.defaults.headers.common['Authorization'] = `Bearer ${newToken}`;
  };

  const clearAuth = () => {
    user.value = null;
    token.value = null;
    isAuthenticated.value = false;
    localStorage.removeItem('hephaestus_token');
    localStorage.removeItem('hephaestus_user');
    delete axios.defaults.headers.common['Authorization'];
  };

  const fetchUser = async () => {
    if (!token.value) return null;
    try {
      const res = await axios.get('/api/v1/auth/me');
      if (res.data.success) {
        user.value = res.data.data;
        localStorage.setItem('hephaestus_user', JSON.stringify(res.data.data));
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

  // RBAC permission check helper
  const can = (feature: string, action: 'read' | 'manage' = 'read'): boolean => {
    if (!user.value) return false;
    // Superadmin has full unrestricted access
    if (user.value.role?.toUpperCase() === 'ADMIN') return true;

    const perms = user.value.permissions || {};
    if (perms['*'] === 'manage') return true;

    const perm = perms[feature] || 'none';
    if (action === 'read') {
      return perm === 'read' || perm === 'manage';
    }
    if (action === 'manage') {
      return perm === 'manage';
    }
    return false;
  };

  return {
    user,
    token,
    isAuthenticated,
    setAuth,
    clearAuth,
    fetchUser,
    logout,
    can,
  };
});
