# UI & Page Header Guidelines

## 1. Page Title & Header Convention (Standar Judul Halaman)
Ketika menambahkan halaman atau fitur baru di antarmuka web (frontend), selalu ikuti konvensi header dan judul halaman yang seragam dengan halaman-halaman yang sudah ada di Hephaestus Control Panel (HCP) seperti *Prometheus Config*, *Data Prepper Pipelines*, *Add Connections*, dan *Database Backup Manager*:

- **DILARANG menambahkan icon pada Title Halaman**:
  - Jangan menyematkan icon Lucide/SVG, kotak badge icon bergradien, atau ornamen visual apa pun di dalam atau di samping teks judul `<h1>`.
  - Jangan menambahkan pill/badge tambahan (seperti badge `AGENT FLEET`, `NEW`, dsb.) di samping judul halaman.
- **Format Header Standar**:
  ```html
  <!-- Header Section -->
  <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-slate-200 dark:border-[#1b2234] pb-4">
    <div>
      <h1 class="text-xl font-bold text-slate-900 dark:text-white tracking-tight">
        Nama Menu / Fitur
      </h1>
      <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
        Deskripsi singkat tentang fungsi dan tujuan halaman fitur ini.
      </p>
    </div>

    <!-- Header Action Buttons (Kanan) -->
    <div class="flex items-center gap-2 shrink-0">
      <!-- Tombol aksi seperti Refresh atau + Add Item -->
    </div>
  </div>
  ```

## 2. Dukungan Light Mode & Dark Mode
- Semua teks editor kode/YAML harus memiliki kontras yang tepat di kedua mode:
  - Gutter nomor baris: `bg-slate-100 dark:bg-[#070a10]` dengan `text-slate-400 dark:text-slate-500`.
  - Textarea/Container: `bg-white dark:bg-[#090d16]` dengan `text-slate-800 dark:text-slate-100`.
- Notifikasi banner harus memiliki timer auto-dismiss (3 detik) agar tidak mengganggu tampilan.
