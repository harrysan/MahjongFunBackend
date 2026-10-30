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

// Join seats a human in the named room. With create, the room must not exist
// yet and is made (its first human becomes host); without it, the room must
// already exist.
func (m *Manager) Join(roomName, playerName string, create bool, c Client) (*Room, engine.Seat, error) {
	roomName = strings.ToUpper(strings.TrimSpace(roomName)) // codes are case-insensitive
	playerName = strings.TrimSpace(playerName)
	if !validName(roomName, MaxRoomNameLen) || !validName(playerName, MaxPlayerNameLen) {
		return nil, -1, ErrBadName
	}
	for {
		m.mu.Lock()
		r := m.rooms[roomName]
		switch {
		case r == nil && !create:
			m.mu.Unlock()
			return nil, -1, ErrRoomNotFound
		case r != nil && create:
			m.mu.Unlock()
			return nil, -1, ErrRoomExists
		case r == nil:
			r = newRoom(roomName, m, m.opts)
			m.rooms[roomName] = r
		}
		m.mu.Unlock()

		s, err := r.join(playerName, c)
		if err == ErrRoomClosed {
			if !create {
				return nil, -1, ErrRoomNotFound
			}
			continue // a brand-new room cannot close before we join; just retry
		}
		if err != nil {
			return nil, -1, err
		}
		return r, s, nil
	}
}

// Rejoin puts a player whose connection dropped back in their seat. token is
// the one they got in room_state.
func (m *Manager) Rejoin(roomName, token string, c Client) (*Room, engine.Seat, error) {
	roomName = strings.ToUpper(strings.TrimSpace(roomName))
	m.mu.Lock()
	r := m.rooms[roomName]
	m.mu.Unlock()
	if r == nil {
		return nil, -1, ErrRoomNotFound
	}
	s, err := r.rejoin(token, c)
	if err == ErrRoomClosed {
		return nil, -1, ErrRoomNotFound
	}
	if err != nil {
		return nil, -1, err
	}
	return r, s, nil
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
