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
- **Wadah Root Halaman (Keseragaman Margin & No Double Padding)**:
  - Wadah layout utama (`AppLayout.vue`) sudah memiliki padding bawaan pada tag `<main>` (`p-3.5 sm:p-5 md:p-6`).
  - Oleh karena itu, root `<div>` di setiap file halaman (`View.vue`) **DILARANG** menambahkan padding sendiri seperti `p-4 sm:p-6` atau `min-h-screen` karena akan menyebabkan padding atas berlipat ganda (*double padding*).
  - Gunakan struktur standar root container: `<div class="space-y-6 max-w-7xl mx-auto font-sans">` (atau `max-w-[1600px]`). Hal ini memastikan jarak margin/padding top halaman selalu seragam dan presisi di seluruh fitur.

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

### Iconography & Emoticon Guidelines (Aturan Ikon dan Emoticon)
Untuk menjaga tampilan HCP tetap profesional, bersih (*clean enterprise-grade*), dan konsisten:
- **DILARANG MENGGUNAKAN EMOTICON / KARAKTER EMOJI**:
  - Dilarang keras menggunakan emoticon atau karakter emoji Unicode (seperti 🚀, ⚙️, 🟢, 🔴, ⚠️, ✅, 📁, 💻, 📊, dsb.) pada judul halaman, label tombol aksi, card, badge/tag, pesan notifikasi/toast, maupun menu navigasi.
  - Gunakan ikon SVG resmi dari `lucide-vue-next` jika elemen visual memang dibutuhkan.
- **IKON HARUS BERSIH & MONOKROMATIK (Neutral Monochrome Icons)**:
  - Hindari memberikan warna-warni cerah (`text-blue-500`, `text-blue-600`, `text-amber-500`, `text-purple-500`, `text-cyan-400`) pada ikon navigasi sidebar, tombol aksi toolbar (seperti *Refresh*, *Restart*, *Presets*, *History*, *Edit*, *Copy*), search bar, command palette, ataupun header card.
  - Gunakan warna netral: `text-slate-400`, `text-slate-500`, `text-slate-600 dark:text-slate-400`, atau biarkan mewarisi `currentColor`.
  - Ikon tombol hanya boleh berganti kontras saat hover atau active bersamaan dengan teks induknya (`group-hover:text-...` atau warna standar teks tombol).
- **DILARANG MENAMBAHKAN BULLET DOT BERWARNA DEKORATIF**:
  - Jangan menambahkan bulatan/dot berwarna (`bg-blue-600`, `bg-[#4274D9]`, dsb.) di samping judul halaman, breadcrumb top navigation bar, maupun link submenu sidebar.
  - Gunakan indentasi vertikal (`pl-6`), border tipis netral (`border-l border-slate-200 dark:border-[#1b2234]`), atau teks bersih.
- **PENGECUALIAN WARNA HANYA UNTUK STATUS SEMANTIK KRITIS (Critical Semantic Only)**:
  - Warna (hijau, merah, kuning/amber) **HANYA** boleh digunakan secara terbatas untuk indikator status sistem nyata:
    - Dot status koneksi host/agent: Aktif/Healthy (`emerald-500`), Failed/Down (`rose-500`), Inactive/Unreachable (`slate-400` / `amber-500`).
    - Badge status teks (misal badge `ACTIVE` hijau, `FAILED` merah).
    - Tombol aksi destruktif permanen (misal tombol Hapus berwarna merah `text-rose-600` / `bg-rose-600`).
    - Banner notifikasi feedback (Success -> border hijau, Error -> border merah).
  - Di luar status semantik di atas, seluruh komponen UI dan ikon wajib menggunakan palet monokromatik netral.
