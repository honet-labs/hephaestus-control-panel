import { defineStore } from 'pinia';
import { ref } from 'vue';
import axios from 'axios';

// Ensure withCredentials is true so HttpOnly session cookie is always sent (H-02)
axios.defaults.withCredentials = true;

export interface User {
  id: number;
  username: string;
  role: string;
  permissions?: Record<string, string>;
  forcePasswordChange: boolean;
}

export const useAuthStore = defineStore('auth', () => {
  const savedUserStr = sessionStorage.getItem('hcp_user');
  let initialUser: User | null = null;
  if (savedUserStr) {
    try {
      initialUser = JSON.parse(savedUserStr);
    } catch (_) {}
  }

  const savedToken = sessionStorage.getItem('hcp_session');
  const user = ref<User | null>(initialUser);
  const token = ref<string | null>(savedToken);
  const isAuthenticated = ref<boolean>(!!savedToken || !!initialUser);

  if (savedToken) {
    axios.defaults.headers.common['Authorization'] = `Bearer ${savedToken}`;
  }

  const setAuth = (newUser: User, newToken?: string) => {
    user.value = newUser;
    token.value = newToken || null;
    isAuthenticated.value = true;
    sessionStorage.setItem('hcp_user', JSON.stringify(newUser));
    if (newToken) {
      sessionStorage.setItem('hcp_session', newToken);
      axios.defaults.headers.common['Authorization'] = `Bearer ${newToken}`;
    }
  };

  const clearAuth = () => {
    user.value = null;
    token.value = null;
    isAuthenticated.value = false;
    sessionStorage.removeItem('hcp_user');
    sessionStorage.removeItem('hcp_session');
    delete axios.defaults.headers.common['Authorization'];
  };

  const fetchUser = async () => {
    try {
      const res = await axios.get('/api/v1/auth/me');
      if (res.data && res.data.success) {
        user.value = res.data.data;
        isAuthenticated.value = true;
        sessionStorage.setItem('hcp_user', JSON.stringify(res.data.data));
        return user.value;
      }
    } catch (err: any) {
      // ONLY clear auth if the server explicitly returned 401 Unauthorized or 403 Forbidden
      if (err.response && (err.response.status === 401 || err.response.status === 403)) {
        clearAuth();
        return null;
      }
      // For network errors or temporary 5xx downtime, keep existing cached user state from sessionStorage
      console.warn('Backend temporarily unreachable, retaining cached auth session:', err?.message);
      return user.value;
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
