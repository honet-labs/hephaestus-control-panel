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
  // Session authentication is stored in HttpOnly; Secure; SameSite=Strict cookie (H-02)
  // Token and user state in JS are stored strictly in-memory (no localStorage / sessionStorage token)
  const user = ref<User | null>(null);
  const token = ref<string | null>(null);
  const isAuthenticated = ref<boolean>(false);
  const isInitialized = ref<boolean>(false);

  const setAuth = (newUser: User, newToken?: string) => {
    user.value = newUser;
    token.value = newToken || null;
    isAuthenticated.value = true;
    isInitialized.value = true;
    if (newToken) {
      axios.defaults.headers.common['Authorization'] = `Bearer ${newToken}`;
    }
  };

  const clearAuth = () => {
    user.value = null;
    token.value = null;
    isAuthenticated.value = false;
    isInitialized.value = true;
    delete axios.defaults.headers.common['Authorization'];
  };

  const fetchUser = async () => {
    try {
      const res = await axios.get('/api/v1/auth/me');
      if (res.data && res.data.success) {
        user.value = res.data.data;
        isAuthenticated.value = true;
        return user.value;
      }
    } catch (err: any) {
      // ONLY clear auth if the server explicitly returned 401 Unauthorized or 403 Forbidden
      if (err.response && (err.response.status === 401 || err.response.status === 403)) {
        clearAuth();
        return null;
      }
      console.warn('Auth check non-fatal error:', err?.message);
      return user.value;
    } finally {
      isInitialized.value = true;
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
    isInitialized,
    setAuth,
    clearAuth,
    fetchUser,
    logout,
    can,
  };
});
