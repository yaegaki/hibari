package hibari

import (
	"context"
	"fmt"
	"sync"
)

// RoomMap is map of room
type RoomMap map[string]Room

// Manager management rooms
type Manager interface {
	RoomMap() RoomMap
	RoomInfoAll() <-chan RoomInfo
	Authenticate(ctx context.Context, id, secret string) (User, error)
	Negotiate(ctx context.Context, trans ConnTransport) (context.Context, error)
	GetOrCreateRoom(ctx context.Context, id string) (Room, error)
	NotifyRoomClosed(id string)
	Shutdown()
}

type manager struct {
	option     ManagerOption
	mu         *sync.Mutex
	allocator  RoomAllocator
	roomMap    RoomMap
	shutdownCh chan struct{}
}

// ManagerOption configure manager
type ManagerOption struct {
	Authenticator Authenticator
	Negotiator    Negotiator
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

func (m *manager) RoomMap() RoomMap {
	roomMap := RoomMap{}
	m.mu.Lock()
	defer m.mu.Unlock()

	for k, v := range m.roomMap {
		roomMap[k] = v
	}

	return roomMap
}

func (m *manager) RoomInfoAll() <-chan RoomInfo {
	resultCh := make(chan RoomInfo)

	var wg sync.WaitGroup
	roomMap := m.RoomMap()
	for _, v := range roomMap {
		wg.Add(1)
		room := v

		go func() {
			roomInfo, err := room.RoomInfo()
			if err != nil {
				resultCh <- roomInfo
			}

			wg.Done()
		}()
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	return resultCh
}

func (m *manager) Authenticate(ctx context.Context, id, secret string) (User, error) {
	if m.option.Authenticator == nil {
		return User{ID: id, Name: id}, nil
	}

	return m.option.Authenticator.Authenticate(ctx, id, secret)
}

func (m *manager) Negotiate(ctx context.Context, trans ConnTransport) (context.Context, error) {
	if m.option.Negotiator == nil {
		return ctx, nil
	}

	return m.option.Negotiator.Negotiate(ctx, trans)
}

func (m *manager) GetOrCreateRoom(ctx context.Context, id string) (Room, error) {
	if id != "" {
		m.mu.Lock()
		r, ok := m.roomMap[id]
		m.mu.Unlock()

		if ok {
			return r, nil
		}
	}

	newRoom, err := m.allocator.Alloc(ctx, id, m)
	if err != nil {
		return nil, err
	}

	// allow modify roomID by RoomAllocator
	// if RoomAllocator modified roomID, must modify to unique
	id = newRoom.ID()

	if id == "" {
		return nil, fmt.Errorf("Invalid RoomID")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.roomMap[id]

	if ok {
		return r, nil
	}

	m.roomMap[id] = newRoom
	go newRoom.Run()

	return newRoom, nil
}

func (m *manager) NotifyRoomClosed(id string) {
	m.allocator.Free(id)

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

	chs := make([]<-chan struct{}, 0, len(roomMap))
	for _, r := range roomMap {
		chs = append(chs, r.Shutdown().Done())
	}

	for _, ch := range chs {
		<-ch
	}
}
