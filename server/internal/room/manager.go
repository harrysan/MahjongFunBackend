package room

import (
	"strings"
	"sync"
	"unicode/utf8"

	"mahjong/server/internal/engine"
)

// Manager keeps every open room by name.
type Manager struct {
	opts  Options
	mu    sync.Mutex
	rooms map[string]*Room
}

func NewManager(opts Options) *Manager {
	return &Manager{opts: opts, rooms: map[string]*Room{}}
}

// Join seats a human in the named room, creating the room if needed.
// The first human in a room becomes its host.
func (m *Manager) Join(roomName, playerName string, c Client) (*Room, engine.Seat, error) {
	roomName = strings.TrimSpace(roomName)
	playerName = strings.TrimSpace(playerName)
	if !validName(roomName, MaxRoomNameLen) || !validName(playerName, MaxPlayerNameLen) {
		return nil, -1, ErrBadName
	}
	for {
		m.mu.Lock()
		r := m.rooms[roomName]
		if r == nil {
			r = newRoom(roomName, m, m.opts)
			m.rooms[roomName] = r
		}
		m.mu.Unlock()

		s, err := r.join(playerName, c)
		if err == ErrRoomClosed {
			continue // the room closed while we were joining; make a fresh one
		}
		if err != nil {
			return nil, -1, err
		}
		return r, s, nil
	}
}

// Count is the number of open rooms.
func (m *Manager) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.rooms)
}

func (m *Manager) remove(r *Room) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.rooms[r.name] == r {
		delete(m.rooms, r.name)
	}
}

func validName(s string, max int) bool {
	return s != "" && utf8.RuneCountInString(s) <= max
}
