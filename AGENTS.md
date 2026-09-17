# Hephaestus Control Panel (HCP) Agent Guidelines

## Frontend & UI Guidelines

### Page Title & Header Conventions
Setiap kali menambahkan halaman atau fitur baru di frontend (`web/`):
- **JANGAN menambahkan icon pada Title Halaman (`<h1>`)**:
  - Jangan menambahkan icon Lucide/SVG, kotak rounded bergradien, atau ornamen visual apa pun di samping atau di dalam judul halaman.
  - Jangan menambahkan badge status atau badge teks tambahan (seperti `AGENT FLEET`, `FEATURE`, dsb.) di dalam judul `<h1>`.
  - Judul harus bersih hanya teks murni, mencontoh halaman-halaman yang sudah ada di HCP (*Prometheus Config*, *Data Prepper Pipelines*, *Add Connections*, *Database Backup Manager*).
- **Struktur Standar Header**:
  ```html
  <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-slate-200 dark:border-[#1b2234] pb-4">
    <div>
      <h1 class="text-xl font-bold text-slate-900 dark:text-white tracking-tight">
        Nama Menu / Fitur
      </h1>
      <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
        Deskripsi singkat kegunaan fitur.
      </p>
    </div>
    <div class="flex items-center gap-2 shrink-0">
      <!-- Tombol aksi (Refresh, Add, dll.) -->
    </div>
  </div>
  ```

### Standard Delete Confirmation Modal (Konfirmasi Hapus Wajib Modal Popup)
Setiap menu atau fitur di HCP jika ingin **MENGHAPUS (DELETE)** resource atau data apa pun:
- **DILARANG MENGGUNAKAN `window.confirm()` atau alert browser bawaan**.
- **WAJIB memunculkan modal popup konfirmasi** dengan standar tampilan HCP:
  1. **Backdrop**: Layar belakang gelap transparan dengan efek blur:
     ```html
     <div class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in">
     ```
  2. **Modal Card**:
     ```html
     <div class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-sm shadow-2xl p-5 space-y-4 text-center">
     ```
  3. **Icon Lingkaran Merah di Tengah Atas**:
     ```html
     <div class="w-12 h-12 rounded-full bg-rose-500/10 text-rose-500 flex items-center justify-center mx-auto">
       <Trash2 class="w-6 h-6" />
     </div>
     ```
  4. **Judul & Teks Penjelasan**:
     ```html
     <div class="space-y-1">
       <h3 class="text-sm font-bold text-slate-900 dark:text-white">Delete [Nama Resource]?</h3>
       <p class="text-xs text-slate-500 dark:text-slate-400">
         Are you sure you want to remove <strong class="text-slate-800 dark:text-slate-200">{{ itemName }}</strong>? This action cannot be undone.
       </p>
     </div>
     ```
  5. **Tombol Aksi**:
     ```html
     <div class="flex items-center justify-center gap-2 pt-2">
       <button
         @click="showDeleteModal = false"
         class="px-3 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
       >
         Cancel
       </button>
       <button
         @click="executeDelete"
         :disabled="deleting"
         class="px-4 py-1.5 bg-rose-600 hover:bg-rose-500 text-white rounded-lg text-xs font-bold transition cursor-pointer disabled:opacity-50"
       >
         {{ deleting ? 'Deleting...' : 'Confirm Delete' }}
       </button>
     </div>
     ```

### Light Mode & Dark Mode Standards
- Pastikan semua elemen UI (termasuk editor kode YAML, gutter nomor baris, input, dan modal) memiliki styling kontras yang serasi dan terbaca jelas baik pada mode terang (`html.light`) maupun mode gelap (`dark`).
- Jangan menetapkan warna gelap/hitam statis tanpa varian light mode.
- Banner notifikasi feedback harus otomatis hilang (*auto-dismiss*) dalam 3 detik (`3000ms`).
