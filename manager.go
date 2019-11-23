package hibari

import (
	"sync"
)

// Manager management rooms
type Manager interface {
	GetOrCreateRoom(id string) (Room, error)
	NotifyRoomClosed(id string)
	Shutdown()
}

type manager struct {
	option     ManagerOption
	mu         *sync.Mutex
	allocator  RoomAllocator
	roomMap    map[string]Room
	shutdownCh chan struct{}
}

// ManagerOption configure manager
type ManagerOption struct {
}

// NewManager creates a new Manager
func NewManager(ra RoomAllocator, option *ManagerOption) Manager {
	if ra == nil {
		ra = internalRoomAllocator{}
	}

	if option == nil {
		option = &ManagerOption{}
	}

	m := &manager{
		option:    *option,
		mu:        &sync.Mutex{},
		allocator: ra,
		roomMap:   map[string]Room{},
	}

	return m
}

func (m *manager) GetOrCreateRoom(id string) (Room, error) {
	m.mu.Lock()
	r, ok := m.roomMap[id]
	m.mu.Unlock()

	if ok {
		return r, nil
	}
	newRoom, err := m.allocator.Alloc(id, m)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok = m.roomMap[id]

	if ok {
		return r, nil
	}

	m.roomMap[id] = newRoom
	go newRoom.Run()

	return newRoom, nil
}

func (m *manager) NotifyRoomClosed(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	room, ok := m.roomMap[id]
	if !ok {
		return
	}

	if !room.Closed() {
		return
	}

	delete(m.roomMap, id)
}

func (m *manager) Shutdown() {
	m.mu.Lock()
	roomMap := m.roomMap
	m.roomMap = map[string]Room{}
	m.mu.Unlock()

	chs := make([]chan struct{}, 0, len(roomMap))
	for _, r := range roomMap {
		chs = append(chs, r.Shutdown())
	}

	for _, ch := range chs {
		<-ch
	}
}
