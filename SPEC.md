# SPEC: Mahjong Web Game

Game mahjong berbasis web, 4 pemain (manusia atau bot). Tanpa sistem skor.
Dokumen ini adalah acuan utama. Baca file ini sebelum mengerjakan task apa pun.

Semua aturan di sini sudah dikonfirmasi pemilik proyek, kecuali yang bertanda **[ASUMSI]**.

---

## 1. Ringkasan

- Mode: **solo** (1 manusia + 3 bot) atau **multiplayer** (2-4 manusia, sisanya bot).
- Solo = room biasa dengan bot. Tidak ada kode khusus solo.
- Server adalah satu-satunya pemegang state game (server-authoritative).
- Klien hanya mengirim aksi dan menampilkan state.

## 2. Tech Stack

| Bagian | Teknologi |
|---|---|
| Backend | Go 1.22+, `net/http`, `github.com/coder/websocket` v1.8.12 (versi terakhir yang mendukung Go 1.22) |
| Frontend | Vue 3 (Composition API), Pinia, Vite, TypeScript |
| Komunikasi | WebSocket, pesan JSON |
| Penyimpanan | State game di memori. Database ditunda sampai ada kebutuhan akun/riwayat |

## 3. Struktur Proyek

```
mahjong/
  SPEC.md
  server/
    cmd/server/main.go
    cmd/sim/      # simulasi bot vs bot
    internal/
      engine/     # aturan game murni, TANPA I/O, TANPA goroutine
      bot/        # logika bot, memakai engine
      room/       # room, kursi, siklus game, timer
      ws/         # handler WebSocket, encode/decode pesan
  web/
    src/
      stores/     # Pinia: connection, game state
      components/ # Tile, Hand, DiscardPile, ActionBar, Seat
      views/      # Lobby, WaitingRoom, Table
```

Aturan penting: paket `engine` tidak boleh mengimpor paket lain dari proyek.
Semua logika aturan harus bisa di-test tanpa jaringan.

## 4. Aturan Permainan

### 4.1 Set tile (144 tile)

| Jenis | Isi | Jumlah |
|---|---|---|
| Titik (Dots) | 1-9, masing-masing 4 | 36 |
| Bambu (Bamboo) | 1-9, masing-masing 4 | 36 |
| Karakter (Characters) | 1-9, masing-masing 4 | 36 |
| Angin (Winds) | Timur, Selatan, Barat, Utara, masing-masing 4 | 16 |
| Naga (Dragons) | Merah, Hijau, Putih, masing-masing 4 | 12 |
| Bunga (Flowers) | 1-4 | 4 |
| Musim (Seasons) | 1-4 | 4 |

Tile bunga dan musim disebut **bonus tile**. Bonus tile tidak masuk set di tangan.

### 4.2 Jalannya game

1. Kocok 144 tile menjadi wall. Bagi kursi: Timur (dealer), Selatan, Barat, Utara, searah jarum jam.
2. Setiap pemain menerima **13 tile**. Dealer langsung mengambil tile ke-14 dan mulai dengan membuang.
3. **Bonus tile:** jika pemain mendapat bonus tile (saat bagi atau saat ambil), tile itu ditaruh terbuka di depannya, lalu pemain mengambil pengganti dari **ujung belakang wall**. Ulangi jika pengganti juga bonus tile.
4. **Giliran:** ambil 1 tile dari depan wall, lalu buang 1 tile (kecuali menang).
5. Setelah tile dibuang, terbuka **jendela klaim** (lihat 4.3).
6. Game berakhir jika ada yang **hu**, atau wall habis (**seri**, tidak ada pemenang).

### 4.3 Klaim tile buangan

| Aksi | Siapa yang boleh | Syarat |
|---|---|---|
| Chi | Hanya pemain **setelah** pembuang (giliran berikutnya) | Membentuk urutan 3 tile satu suit dengan 2 tile di tangan. Angin dan naga tidak bisa di-chi |
| Pung | Siapa saja | Punya 2 tile yang sama |
| Kong | Siapa saja | Punya 3 tile yang sama. Setelah kong, ambil 1 tile pengganti dari belakang wall |
| Hu | Siapa saja | Tile buangan melengkapi tangan |

Prioritas jika beberapa pemain klaim bersamaan: **Hu > Pung/Kong > Chi**.
Jika lebih dari satu pemain hu pada tile yang sama: pemain yang paling dekat
dengan pembuang (searah giliran) yang menang. Yang lain tidak hu.

Setelah klaim pung atau chi, pemain mengambil tile buangan, menaruh set terbuka,
lalu **membuang 1 tile** (tidak mengambil dari wall). Giliran berlanjut dari pemain tersebut.

Jika semua pemain pass, giliran lanjut normal ke pemain berikutnya.

### 4.4 Kong dari tangan sendiri

- **Kong tertutup:** 4 tile sama di tangan. Boleh diumumkan di giliran sendiri.
- **Kong tambahan:** sudah punya pung terbuka, lalu mengambil tile ke-4. Ditambahkan ke pung tersebut.
- Setelah kong, ambil 1 tile pengganti dari belakang wall.
- Tidak ada merampok kong: tile kong tambahan pemain lain tidak bisa di-hu.

### 4.5 Hu (menang)

Tangan lengkap = **4 set + 1 pasangan** (14 tile, kong dihitung 1 set).
Set = urutan 3 tile satu suit, atau 3 tile sama (pung), atau 4 tile sama (kong).
Bonus tile tidak dihitung.

- **Hu dari wall:** setelah mengambil tile sendiri.
- **Hu dari buangan:** tile buangan pemain lain melengkapi tangan.
- Tangan spesial (tujuh pasang, 13 yatim, dan sebagainya) **tidak** didukung.
- Tidak ada syarat minimal poin/fan. Tangan lengkap = boleh hu.
- Tidak ada skor. Pemenang ditampilkan, lalu bisa mulai ronde baru.

### 4.6 Timer

- Giliran buang: 30 detik. Jika habis, server membuang tile yang baru diambil secara otomatis
  (setelah chi/pung, yang dibuang adalah tile terakhir yang boleh dibuang).
- Jendela klaim: 10 detik. Jika habis, dianggap pass.
- Pemain yang terputus: bot mengambil alih kursinya sampai ia kembali (`rejoin_room`).
  Di ruang tunggu kursinya disimpan 60 detik. Jika tidak ada pemain manusia yang tersambung,
  permainan dibekukan (timer dan bot berhenti) dan room ditutup setelah 5 menit.

Nilai timer disimpan sebagai konstanta di satu tempat agar mudah diubah.
Nilai di atas adalah default; host boleh mengubahnya per room lewat `update_settings`
(giliran 5-120 detik, klaim 3-60 detik, atau tanpa batas).

### 4.7 Ronde dan dealer

- Ronde pertama: dealer = kursi 0 (Timur).
- Setelah ada yang hu, **pemenang melempar dadu** (2 dadu, total 2-12) untuk menentukan dealer ronde berikutnya.
- Hitung **searah jarum jam** mulai dari pemenang sebagai hitungan ke-1. Pemain yang jatuh pada hitungan terakhir menjadi dealer (Timur) ronde berikutnya.
  Rumus: `dealer_baru = (kursi_pemenang + total_dadu - 1) % 4`.
  Contoh: pemenang di kursi 2, dadu total 5, maka dealer baru = (2 + 5 - 1) % 4 = kursi 2 (pemenang sendiri). Dadu total 6 maka kursi 3.
- Dadu dilempar oleh **server** (acak), klien hanya menampilkan animasinya.
- Ronde seri (wall habis): dealer tetap sama **[ASUMSI]**.
- Tidak ada skor, jadi tidak ada angin ronde atau angin kursi yang berpengaruh.

## 5. Representasi Tile (Go)

```go
// Kind: jenis tile. 0-33 tile biasa, 34-41 bonus.
// 0-8 Dots 1-9, 9-17 Bamboo 1-9, 18-26 Characters 1-9,
// 27-30 Winds E,S,W,N, 31-33 Dragons Red,Green,White,
// 34-37 Flowers 1-4, 38-41 Seasons 1-4
type Kind uint8

type Tile struct {
    ID   uint8 // 0-143, unik per fisik tile
    Kind Kind  // ID / 4 untuk tile biasa
}
```

Untuk hitungan di tangan, gunakan `[34]uint8` (jumlah per Kind) agar deteksi menang cepat.

## 6. Arsitektur Engine

Engine berupa state machine murni:

```go
type Game struct { /* wall, hands, melds, discards, bonus, turn, phase */ }

func NewGame(seed int64) *Game
func (g *Game) Apply(seat Seat, a Action) ([]Event, error)
func (g *Game) LegalActions(seat Seat) []Action
func (g *Game) ViewFor(seat Seat) PlayerView // sembunyikan tangan pemain lain
```

- `seed` menentukan kocokan, sehingga game bisa direproduksi di test.
- `Apply` memvalidasi semuanya. Aksi ilegal mengembalikan error, state tidak berubah.
- Fase: `Drawing`, `Discarding`, `ClaimWindow`, `Ended`.
- `Game` menyimpan `Dealer Seat`. Fungsi `NextDealer(winner Seat, dice [2]int) Seat` ada di engine dan di-test (lihat bagian 10).
- Package `room` yang mengatur timer dan goroutine, bukan engine.

## 7. Protokol WebSocket

Semua pesan: `{"type": "...", "data": {...}}`.

### Klien ke server

| type | data | Keterangan |
|---|---|---|
| `join_room` | `{room, name, create?}` | `create: true` membuat room baru (gagal jika kode sudah dipakai); tanpa `create` hanya masuk ke room yang sudah ada. Kode room tidak peka huruf besar/kecil |
| `add_bot` | `{seat}` | Hanya host, sebelum mulai. Isi kursi kosong dengan bot |
| `remove_bot` | `{seat}` | Hanya host, sebelum mulai. Kosongkan kursi bot |
| `rejoin_room` | `{room, token}` | Kembali ke kursi sendiri setelah koneksi putus/refresh. `token` didapat dari `room_state` |
| `leave_room` | `{}` | Keluar permanen. Saat ronde berjalan kursi diambil alih bot dan token tidak berlaku lagi |
| `update_settings` | `{bot_delay_min_ms, bot_delay_max_ms, turn_seconds, claim_seconds}` | Hanya host, tidak saat ronde berjalan. Timeout 0 = tanpa batas |
| `start_game` | `{}` | Hanya host |
| `discard` | `{tile_id}` | Buang tile |
| `claim` | `{action: "chi"\|"pung"\|"kong"\|"hu"\|"pass", tiles?: [id,id]}` | Saat jendela klaim |
| `self_action` | `{action: "kong"\|"hu", tile_id?}` | Kong sendiri atau hu dari wall |
| `next_round` | `{}` | Host memulai ronde berikutnya. Server melempar dadu dan menentukan dealer |

### Server ke klien

| type | Keterangan |
|---|---|
| `room_state` | Daftar kursi (termasuk `connected`), siapa host, status, pengaturan room, dan `token` milik penerima |
| `game_state` | Snapshot lengkap untuk pemain ini (`PlayerView`) + `time_left_ms`, `time_limit_ms` |
| `event` | Kejadian: `drew`, `discarded`, `claimed`, `bonus`, `turn`, `game_end`, `dice_rolled` (`{dice: [a,b], new_dealer}`) |
| `claim_window` | Aksi yang boleh dilakukan pemain + sisa waktu |
| `error` | `{code, message}` untuk aksi ditolak |

Contoh `game_state` (disederhanakan):

```json
{
  "type": "game_state",
  "data": {
    "seat": 0,
    "phase": "Discarding",
    "turn": 0,
    "hand": [{"id": 12, "kind": 3}],
    "melds": [[{"seat": 0, "type": "pung", "tiles": []}], [], [], []],
    "bonus": [[], [], [], []],
    "discards": [[], [], [], []],
    "hand_counts": [13, 13, 13, 13],
    "wall_remaining": 83,
    "legal_actions": [{"action": "discard", "tile_ids": [12]}]
  }
}
```

Klien tidak pernah menerima isi tangan pemain lain.

## 8. Bot

**v1 (heuristik sederhana):**
- Buang: tile yang paling tidak berguna (tidak punya pasangan atau tetangga satu suit). Angin dan naga tunggal dibuang lebih dulu.
- Klaim: selalu hu jika bisa. Pung jika membantu. Chi jarang. Selain itu pass.
- Bot punya jeda acak 0.5-1.5 detik agar terasa natural.

Bot memakai `LegalActions` dari engine, tidak punya aturan sendiri.

## 9. Frontend

Halaman:
- **Lobby:** nama, main solo, buat room, gabung dengan kode atau tautan undangan (`?room=KODE`).
- **Ruang tunggu:** kode room, daftar kursi, host menambah/menghapus bot dan mengubah pengaturan, lalu mulai.
- **Table:** 4 kursi (diri sendiri di bawah), tangan, tile buangan per pemain, set terbuka, bonus tile, sisa wall, tombol aksi (Chi/Pung/Kong/Hu/Pass), penghitung waktu.

Tile dirender dengan gambar PNG di `web/src/assets/tiles` (pemetaan nama file ada di `web/src/tiles.ts`).
State hanya dari Pinia store yang diisi pesan server. Klien tidak menghitung aturan.

## 10. Persyaratan Test

Wajib ada unit test (`go test ./...`) untuk:
- Jumlah dan komposisi 144 tile, kocokan deterministik dengan seed sama.
- Deteksi hu: contoh tangan menang dan tidak menang (minimal 30 kasus).
- Validasi chi, pung, kong, termasuk kasus ilegal.
- Prioritas klaim (hu > pung > chi, beberapa pemain).
- Bonus tile: pengganti dari belakang wall, berantai.
- Wall habis menghasilkan seri.
- Penentuan dealer berikutnya dari pemenang dan total dadu (semua total 2-12, semua kursi pemenang).
- Tidak ada merampok kong.
- Simulasi 1000 game bot vs bot tanpa panic dan semua selesai.

## 11. Roadmap

Perkiraan untuk 1,5-2 jam per malam (sekitar 10-14 jam per minggu).

### 11.1 Tahap 1: game bisa dimainkan (selesai)

| Milestone | Isi | Status |
|---|---|---|
| M0 | Setup repo, SPEC final | Selesai |
| M1 | Engine: tile, wall, bagi, bonus, buang, ambil, deteksi hu + test | Selesai |
| M2 | Engine: chi/pung/kong, prioritas klaim, seri + test | Selesai |
| M3 | Bot v1 + simulasi bot vs bot | Selesai |
| M4 | WebSocket + UI meja, solo vs 3 bot **bisa dimainkan** | Selesai |
| M5 | Lobby, multiplayer manusia | Selesai |
| M6 | Reconnect, bot pengganti, timer, polish, deploy | Selesai |

### 11.2 Tahap 2: rencana berikutnya

Urutan disusun dari yang paling aman (tidak mengubah aturan) ke yang paling besar.
Milestone bertanda **[PERLU KEPUTUSAN]** mengubah aturan atau arsitektur; jawab pertanyaannya
di bagian 13 dulu sebelum mulai mengerjakan.

| Milestone | Isi | Perkiraan |
|---|---|---|
| M7 | Stabilitas dan test otomatis | Minggu 11-12 |
| M8 | Pengalaman bermain di meja | Minggu 12-14 |
| M9 | Fitur room dan sosial | Minggu 14-16 |
| M10 | Bot v2 dan tingkat kesulitan | Minggu 16-18 |
| M11 | Skor dan aturan opsional **[PERLU KEPUTUSAN]** | Minggu 18-21 |
| M12 | Persistensi, akun, riwayat **[PERLU KEPUTUSAN]** | Minggu 21-25 |

**M7: Stabilitas dan test otomatis**
- CI (misalnya GitHub Actions): `go vet`, `go test ./...`, `go run ./cmd/sim`, `npm run build` di setiap push.
- Test end-to-end di browser (misalnya Playwright): main solo sampai ronde selesai, dua pemain di room yang sama,
  refresh di tengah ronde lalu kembali ke kursi.
- Unit test frontend (Vitest) untuk store `game`: sesi, reconnect, pemetaan pesan server.
- Batas penyalahgunaan: ukuran pesan maksimal, batas pesan per detik per koneksi, batas jumlah room per server.
- Log terstruktur (`log/slog`) untuk room dibuat/ditutup, error, dan reconnect.
- Selesai jika: CI hijau, test e2e berjalan di CI, server tetap stabil saat dikirimi pesan sampah.

**M8: Pengalaman bermain di meja**
- Animasi ambil, buang, dan klaim; animasi lempar dadu (bagian 4.7 menyebut klien menampilkan animasi dadu).
- Suara (buang, klaim, giliranmu, menang) dengan tombol bisu yang diingat di `localStorage`.
- Tangan bisa diurutkan ulang manual (seret), selain urutan otomatis.
- Tampilan HP: layout landscape yang nyaman, tile cukup besar untuk disentuh.
- PWA: bisa dipasang di layar utama HP, ikon dan layar pembuka.
- Selesai jika: satu ronde penuh nyaman dimainkan di HP dan desktop tanpa zoom.

**M9: Fitur room dan sosial**
- Chat singkat dan emote di meja dan ruang tunggu (dengan batas panjang dan laju pesan).
- Host bisa mengeluarkan pemain (kick) dan menyerahkan peran host ke pemain lain.
- Penonton: masuk room yang penuh/berjalan tanpa melihat tangan siapa pun.
- Tambahan protokol (nama pesan final ditentukan saat mengerjakan): `chat`, `kick`, `transfer_host`,
  `join_room` dengan `spectate: true`.
- Selesai jika: semua aksi baru tervalidasi di server dan punya test di paket `room`.

**M10: Bot v2 dan tingkat kesulitan**
- Bot menghitung jarak ke tangan lengkap (shanten) dan memilih buangan yang memperbesar peluang menang.
- Bot memperhitungkan tile yang sudah terlihat (buangan, set terbuka) dan menghindari membuang tile berbahaya.
- Tingkat kesulitan per kursi: Mudah (v1), Normal, Sulit. Dipilih host di ruang tunggu.
- Bot tetap hanya memakai `LegalActions` dari engine.
- Selesai jika: simulasi menunjukkan bot Sulit menang lebih sering dari bot v1 (misalnya 1000 game,
  1 bot Sulit vs 3 bot v1), dan simulasi tetap tanpa panic.

**M11: Skor dan aturan opsional [PERLU KEPUTUSAN]**
- Sistem skor per ronde dan total skor sepanjang sesi room.
- Tangan spesial (tujuh pasang, 13 yatim) sebagai opsi room, default mati.
- Riwayat ronde di dalam sesi: siapa menang, dengan tangan apa, berapa poin.
- Engine tetap murni; perhitungan skor diuji dengan tabel kasus seperti deteksi hu (bagian 10).
- Selesai jika: aturan skor tertulis lengkap di bagian 4 dan setiap pola skor punya test.

**M12: Persistensi, akun, riwayat [PERLU KEPUTUSAN]**
- Database (misalnya SQLite untuk mulai, Postgres jika perlu) untuk akun, statistik, dan riwayat game.
- Room yang sedang berjalan bisa bertahan saat server restart/deploy ulang (snapshot state room).
- Akun opsional: tamu tetap bisa main tanpa daftar.
- Statistik pemain (jumlah main, menang, seri) dan replay ronde. Replay cukup menyimpan seed dan
  urutan aksi, karena engine deterministik.
- Selesai jika: restart server tidak mengakhiri room, dan replay menghasilkan state yang sama persis.

## 12. Panduan untuk Roo Code / AI Coding

- Kerjakan **satu task kecil** per sesi. Contoh: "implementasi `DetectWin` di `engine/win.go` sesuai SPEC.md bagian 4.5, lengkap dengan test".
- Jangan ubah paket `engine` tanpa menjalankan `go test ./...`.
- Jangan menambah fitur di luar SPEC ini kecuali diminta. Skor, tangan spesial, dan akun baru boleh
  dikerjakan di milestone-nya (M11, M12) setelah pertanyaan di bagian 13 dijawab.
- Kerjakan milestone tahap 2 sesuai urutan, kecuali pemilik proyek memilih urutan lain.
- Jika aturan ambigu, tanyakan, jangan menebak.
- Komentar dan identifier dalam bahasa Inggris, UI dalam bahasa Indonesia.

## 13. Keputusan Aturan

Sudah dikonfirmasi:

1. Chi hanya dari pemain sebelumnya (tepat sebelum giliran).
2. Dua pemain hu di tile yang sama: yang terdekat dari pembuang menang.
3. Tidak ada tangan spesial.
4. Dealer berikutnya ditentukan lempar dadu oleh pemenang, dihitung searah jarum jam (lihat 4.7).
5. Tidak ada merampok kong.

Masih berupa asumsi:

- Ronde seri: dealer tetap sama.
- Dadu: 2 dadu, pemenang dihitung sebagai hitungan ke-1.

Pertanyaan terbuka untuk tahap 2 (harus dijawab sebelum milestone terkait dimulai):

- **M11, skor:** pakai sistem skor apa (misalnya fan sederhana gaya Hong Kong, atau poin tetap per jenis menang)?
  Apakah hu dari wall dan hu dari buangan dinilai berbeda? Siapa yang membayar saat hu dari buangan:
  hanya pembuang, atau semua pemain?
- **M11, tangan spesial:** tangan mana saja yang didukung, dan apakah aktif per room atau global?
- **M11, angin:** apakah angin ronde dan angin kursi mulai berpengaruh ke skor (saat ini tidak, lihat 4.7)?
- **M12, akun:** login dengan apa (email + kata sandi, atau Google/OAuth)? Apakah tamu tetap boleh main?
- **M12, hosting:** di mana server dan database akan dijalankan? Ini menentukan pilihan SQLite atau Postgres.
