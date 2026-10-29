# Mahjong Web Game

Game mahjong berbasis web (4 pemain, manusia atau bot). Aturan lengkap ada di [SPEC.md](SPEC.md).

## Status

- [x] M0: struktur repo, SPEC final
- [x] M1: engine dasar (tile, wall, bagi, tile bunga, ambil/buang, hu dari wall, penentuan dealer) + test
- [x] M2: chi/pung/kong, jendela klaim, prioritas klaim, hu dari buangan, kong sendiri
- [x] M3: bot v1 (`server/internal/bot`, simulasi: `go run ./cmd/sim`)
- [x] M4: WebSocket + UI meja, solo vs 3 bot bisa dimainkan; tile memakai gambar di `web/src/assets/tiles`; pengaturan waktu bot berpikir, timer giliran dan klaim
- [x] M5: lobby (solo, buat room, gabung via kode/tautan), ruang tunggu, multiplayer manusia
- [ ] M6: reconnect, polish, deploy

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
