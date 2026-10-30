# Mahjong Web Game

Game mahjong berbasis web untuk 4 pemain. Kursi bisa diisi manusia atau bot.
Bisa dimainkan solo melawan 3 bot, atau bersama teman lewat kode room.
Tidak ada sistem skor: yang pertama hu menang ronde.

Aturan lengkap, protokol, dan roadmap ada di [SPEC.md](SPEC.md). Dokumen itu acuan utama proyek.

## Fitur

- **Solo:** 1 pemain melawan 3 bot, langsung mulai dari lobby.
- **Multiplayer:** buat room, bagikan kode atau tautan undangan (`?room=KODE`), kursi kosong bisa diisi bot.
- **Aturan:** chi/pung/kong, kong tertutup dan kong tambahan, hu dari wall atau dari buangan,
  bonus tile (bunga/musim) dengan pengganti dari belakang wall, prioritas klaim, seri saat wall habis,
  dealer berikutnya ditentukan lempar dadu pemenang.
- **Pengaturan per room:** waktu berpikir bot, batas waktu giliran (5-120 detik) dan klaim (3-60 detik),
  atau tanpa batas. Jika waktu habis, server membuang tile otomatis atau menganggap pass.
- **Reconnect:** refresh halaman atau koneksi putus kembali ke kursi yang sama. Selama terputus, bot
  memainkan kursimu. Jika semua pemain manusia terputus, permainan dibekukan; room ditutup setelah 5 menit.
- **Main di jaringan lokal** (Wi-Fi/LAN) dan **deploy** dengan satu image Docker.

## Status

Tahap 1 (M0-M6) selesai. Rencana berikutnya ada di [SPEC.md bagian 11](SPEC.md#11-roadmap).

- [x] M0: struktur repo, SPEC final
- [x] M1: engine dasar (tile, wall, bagi, tile bunga, ambil/buang, hu dari wall, penentuan dealer) + test
- [x] M2: chi/pung/kong, jendela klaim, prioritas klaim, hu dari buangan, kong sendiri
- [x] M3: bot v1 dan simulasi bot vs bot
- [x] M4: WebSocket + UI meja, solo vs 3 bot bisa dimainkan; gambar tile PNG; pengaturan waktu
- [x] M5: lobby (solo, buat room, gabung via kode/tautan), ruang tunggu, multiplayer manusia
- [x] M6: reconnect, bot pengganti, timer, heartbeat, Docker

## Kebutuhan

| Alat | Versi | Untuk |
|---|---|---|
| Go | 1.22+ | server, engine, test |
| Node.js | 20.19+ (atau 22.12+) | frontend |
| Docker | opsional | build dan deploy satu image |

## Menjalankan (development)

Jalankan dua terminal:

```bash
# terminal 1: server Go (WebSocket di ws://localhost:8080/ws)
cd server
go run ./cmd/server

# terminal 2: frontend Vite, /ws diteruskan ke :8080
cd web
npm install            # sekali saja
npm run dev            # buka http://localhost:5173
```

Perubahan di frontend langsung terlihat (hot reload). Perubahan di server butuh restart `go run`,
dan semua room yang sedang berjalan ikut berakhir.

## Menjalankan tanpa Vite (mode build)

```bash
cd web && npm run build                       # hasil di web/dist
cd ../server && go run ./cmd/server -web ../web/dist
```

Buka http://localhost:8080. Server menyajikan halaman dan WebSocket dari satu port.

### Flag server

| Flag | Default | Keterangan |
|---|---|---|
| `-addr` | `:$PORT`, atau `:8080` | Alamat listen |
| `-web` | kosong | Folder hasil build frontend yang disajikan di `/`. Jika diisi, server juga mencetak alamat LAN |
| `-origins` | `localhost:*,127.0.0.1:*` | Host `Origin` lain yang boleh membuka WebSocket (dipisah koma). Halaman dari host yang sama selalu diizinkan |

Endpoint: `/ws` (WebSocket), `/healthz` (health check).

## Test

```bash
cd server
go test ./...                    # semua test: engine, bot, room, ws
go test ./... -v                 # detail per test
go vet ./...
go run ./cmd/sim                 # 1000 game bot vs bot, cek tidak ada panic
go run ./cmd/sim -games 50 -seed 7 -v   # -v mencetak semua event game pertama

cd ../web
npm run typecheck                # cek tipe TypeScript/Vue
npm run build                    # typecheck + build produksi
```

Wajib menjalankan `go test ./...` setiap kali mengubah `server/internal/engine`.

## Main bersama di satu jaringan (Wi-Fi/LAN)

- **Mode build:** `go run ./cmd/server -web ../web/dist` mencetak alamat seperti
  `open on this network: http://192.168.1.14:8080`. Buka alamat itu di **semua** perangkat,
  termasuk komputermu sendiri (bukan `localhost`), agar tautan undangan bisa dipakai teman.
- **Mode dev:** `npm run dev` juga bisa diakses dari jaringan; buka alamat `Network:` yang dicetak Vite.
- Jika teman tidak bisa membuka halaman: izinkan program di Windows Firewall untuk jaringan
  *Private*, dan pastikan profil Wi-Fi bukan *Public*.

## Deploy (Docker)

```bash
docker build -t mahjong-fun .
docker run -p 8080:8080 mahjong-fun      # buka http://localhost:8080
```

- Image berisi server Go + hasil build frontend (sekitar 9 MB, tanpa shell, berjalan sebagai non-root).
- Server memakai env `PORT` jika ada (Render, Fly.io, Railway, Cloud Run, dan lain-lain), selain itu 8080.
- Health check: `GET /healthz`.
- Server berhenti dengan rapi saat menerima SIGTERM.
- Jika di belakang reverse proxy (Nginx, Caddy), pastikan WebSocket di `/ws` diteruskan
  (header `Upgrade`/`Connection`). Pakai HTTPS di production; klien otomatis memakai `wss://`.

## Batasan saat ini

- **State game hanya di memori.** Restart atau deploy ulang mengakhiri semua room.
  Jalankan **satu** instance saja; room tidak dibagi antar-instance.
- **Sesi disimpan per tab** (sessionStorage). Refresh tetap kembali ke kursi, tetapi menutup tab
  berarti tidak bisa kembali. Ini disengaja agar dua tab bisa menjadi dua pemain berbeda.
- Tidak ada skor, tangan spesial, akun, atau riwayat permainan. Semuanya ada di roadmap tahap 2.

## Struktur

```
SPEC.md                  aturan, protokol, roadmap (acuan utama)
Dockerfile               build web + server menjadi satu image
server/
  cmd/server             entry point HTTP + WebSocket
  cmd/sim                simulasi bot vs bot
  internal/engine        aturan game murni (tanpa I/O, tanpa goroutine)
  internal/bot           bot v1, memakai LegalActions dari engine
  internal/room          room, kursi, siklus ronde, timer, bot pengganti, reconnect
  internal/ws            handler WebSocket, encode/decode pesan
web/
  src/stores             Pinia: connection (WebSocket), game (state + sesi)
  src/views              Lobby, WaitingRoom, Table
  src/components         Tile, Hand, DiscardPile, ActionBar, Seat, SettingsForm
  src/assets/tiles       gambar tile PNG
```

Alur data: klien hanya mengirim aksi (`discard`, `claim`, ...). Server memvalidasi lewat engine
dan mengirim balik `game_state` per pemain. Klien tidak pernah menerima isi tangan pemain lain
dan tidak menghitung aturan sendiri. Detail pesan ada di SPEC.md bagian 7.

## Catatan untuk Roo Code / AI coding

- `.rooignore` sudah ada agar `node_modules`, `dist`, dan `.git` tidak dibaca.
- Selalu baca `SPEC.md` dulu, dan kerjakan satu task kecil per sesi.
- Jangan ubah `server/internal/engine` tanpa menjalankan `go test ./...`.
- Komentar dan identifier dalam bahasa Inggris, teks UI dalam bahasa Indonesia.
