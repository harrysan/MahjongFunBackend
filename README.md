# Mahjong Web Game

Game mahjong berbasis web (4 pemain, manusia atau bot). Aturan lengkap ada di [SPEC.md](SPEC.md).

## Status

- [x] M0: struktur repo, SPEC final
- [x] M1: engine dasar (tile, wall, bagi, tile bunga, ambil/buang, hu dari wall, penentuan dealer) + test
- [x] M2: chi/pung/kong, jendela klaim, prioritas klaim, hu dari buangan, kong sendiri
- [x] M3: bot v1 (`server/internal/bot`, simulasi: `go run ./cmd/sim`)
- [x] M4: WebSocket + UI meja, solo vs 3 bot bisa dimainkan; tile memakai gambar di `web/src/assets/tiles`; pengaturan waktu bot berpikir, timer giliran dan klaim
- [x] M5: lobby (solo, buat room, gabung via kode/tautan), ruang tunggu, multiplayer manusia
- [x] M6: reconnect (refresh/koneksi putus kembali ke kursi yang sama), bot pengganti, timer, heartbeat, Docker

## Menjalankan test engine

Butuh Go 1.22 atau lebih baru (https://go.dev/dl/).

```bash
cd server
go test ./...          # jalankan semua test
go test ./... -v       # lihat detail per test
go vet ./...
go run ./cmd/server    # server dasar, cek http://localhost:8080/healthz
```

## Menjalankan game (development)

Butuh Node 20.19+ untuk frontend. Jalankan dua terminal:

```bash
# terminal 1: server Go (WebSocket di ws://localhost:8080/ws)
cd server
go run ./cmd/server

# terminal 2: frontend Vite, /ws diteruskan ke :8080
cd web
npm install            # sekali saja
npm run dev            # buka http://localhost:5173
```

Atau tanpa Vite: `cd web && npm run build`, lalu `cd server && go run ./cmd/server -web ../web/dist`
dan buka http://localhost:8080.

## Deploy (Docker)

```bash
docker build -t mahjong-fun .
docker run -p 8080:8080 mahjong-fun      # buka http://localhost:8080
```

- Image berisi server Go + hasil build frontend (sekitar 9 MB, tanpa shell).
- Server memakai env `PORT` jika ada (Render, Fly.io, Railway, Cloud Run, dan lain-lain), selain itu 8080.
- Health check: `GET /healthz`.
- State game ada di memori: restart/deploy ulang mengakhiri semua room. Jalankan **satu** instance saja.
- Jika di belakang reverse proxy (Nginx, Caddy), pastikan WebSocket di `/ws` diteruskan
  (`Upgrade`/`Connection` header). Pakai HTTPS di production; klien otomatis memakai `wss://`.

### Main bersama di satu jaringan (Wi-Fi/LAN)

- Mode build: `go run ./cmd/server -web ../web/dist` mencetak alamat seperti
  `open on this network: http://192.168.1.14:8080`. Buka alamat itu di **semua** perangkat,
  termasuk komputermu sendiri (bukan `localhost`), agar tautan undangan bisa dipakai teman.
- Mode dev: `npm run dev` juga bisa diakses dari jaringan; buka alamat `Network:` yang dicetak Vite.
- Jika teman tidak bisa membuka halaman: izinkan program di Windows Firewall untuk jaringan
  *Private*, dan pastikan profil Wi-Fi bukan *Public*.

## Struktur

```
server/internal/engine   aturan game murni (tanpa I/O)
server/internal/bot      bot (M3)
server/internal/room     room, kursi, siklus ronde, giliran bot
server/internal/ws       handler WebSocket, encode/decode pesan
web/                     Vue 3 + Pinia + Vite (stores/, components/, views/)
```

## Catatan untuk Roo Code

- `.rooignore` sudah ada agar `node_modules`, `dist`, dan `.git` tidak dibaca.
- Selalu minta Roo membaca `SPEC.md` dulu, dan kerjakan satu task kecil per sesi.
- Jangan ubah `server/internal/engine` tanpa menjalankan `go test ./...`.
