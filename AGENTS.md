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

### Light Mode & Dark Mode Standards
- Pastikan semua elemen UI (termasuk editor kode YAML, gutter nomor baris, input, dan modal) memiliki styling kontras yang serasi dan terbaca jelas baik pada mode terang (`html.light`) maupun mode gelap (`dark`).
- Jangan menetapkan warna gelap/hitam statis tanpa varian light mode.
- Banner notifikasi feedback harus otomatis hilang (*auto-dismiss*) dalam 3 detik (`3000ms`).
