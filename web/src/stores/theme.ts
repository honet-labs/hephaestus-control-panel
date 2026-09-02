import { defineStore } from 'pinia';
import { ref, computed } from 'vue';

export const useThemeStore = defineStore('theme', () => {
  // Read saved theme from localStorage or default to 'dark'
  const saved = localStorage.getItem('hcp_theme') as 'dark' | 'light' | null;
  const currentTheme = ref<'dark' | 'light'>(saved === 'light' ? 'light' : 'dark');

  const isDark = computed(() => currentTheme.value === 'dark');

  const applyTheme = (theme: 'dark' | 'light') => {
    currentTheme.value = theme;
    localStorage.setItem('hcp_theme', theme);
    const root = document.documentElement;
    if (theme === 'dark') {
      root.classList.add('dark');
      root.classList.remove('light');
    } else {
      root.classList.remove('dark');
      root.classList.add('light');
    }
  };

  const toggleTheme = () => {
    const next = currentTheme.value === 'dark' ? 'light' : 'dark';
    applyTheme(next);
  };

  const initTheme = () => {
    applyTheme(currentTheme.value);
  };

  return {
    currentTheme,
    isDark,
    toggleTheme,
    applyTheme,
    initTheme,
  };
});
