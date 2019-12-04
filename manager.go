package hibari

import (
	"context"
	"errors"
	"sync"
)

// RoomMap is map of rooms.
type RoomMap map[string]GoroutineSafeRoom

// Manager management rooms
type Manager interface {
	RoomMap() RoomMap
	RoomInfoAll() <-chan RoomInfo
	Authenticate(ctx context.Context, id, secret string) (User, error)
	Negotiate(ctx context.Context, trans ConnTransport) (context.Context, error)
	GetRoom(id string) (GoroutineSafeRoom, bool)
	GetOrCreateRoom(ctx context.Context, id string) (GoroutineSafeRoom, error)
	NotifyRoomClosed(id string)
	Shutdown()
}

type manager struct {
	option     ManagerOption
	mu         *sync.Mutex
	allocator  RoomAllocator
	roomMap    roomMap
	shutdownCh chan struct{}
}

type roomMap struct {
	m *sync.Map
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
		roomMap:   roomMap{m: &sync.Map{}},
	}

	return m
}

func (m *manager) RoomMap() RoomMap {
	roomMap := RoomMap{}

	m.roomMap.Range(func(id string, r GoroutineSafeRoom) bool {
		roomMap[id] = r
		return true
	})

	return roomMap
}

func (m *manager) RoomInfoAll() <-chan RoomInfo {
	resultCh := make(chan RoomInfo)

	wg := &sync.WaitGroup{}
	m.roomMap.Range(func(id string, r GoroutineSafeRoom) bool {
		wg.Add(1)

		go func() {
			defer wg.Done()
			roomInfo, err := r.SafeRoomInfo()
			if err != nil {
				return
			}
			resultCh <- roomInfo
		}()

		return true
	})

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

func (m *manager) GetRoom(id string) (GoroutineSafeRoom, bool) {
	return m.roomMap.Load(id)
}

func (m *manager) GetOrCreateRoom(ctx context.Context, id string) (GoroutineSafeRoom, error) {
	if id != "" {
		r, ok := m.roomMap.Load(id)

		if ok {
			return r, nil
		}
	}

	newRoom, err := m.allocator.Alloc(ctx, id, m)
	if err != nil {
		return nil, err
	}

	// allow modify roomID by RoomAllocator
	id = newRoom.ID()

	if id == "" {
		return nil, errors.New("invalid RoomID")
	}

	gsRoom, loaded := m.roomMap.LoadOrStore(id, newRoom.ForGoroutine())

	if !loaded {
		go newRoom.Run()
	}

	return gsRoom, nil
}

func (m *manager) NotifyRoomClosed(id string) {
	m.allocator.Free(id)

	room, ok := m.roomMap.Load(id)
	if !ok {
		return
	}

	if !room.Closed() {
		return
	}

	m.roomMap.Delete(id)
}

func (m *manager) Shutdown() {
	m.mu.Lock()
	rm := m.roomMap
	m.roomMap = roomMap{m: &sync.Map{}}
	m.mu.Unlock()

	wg := sync.WaitGroup{}
	rm.Range(func(id string, r GoroutineSafeRoom) bool {
		wg.Add(1)
		ch := r.Close().Done()
		go func() {
			<-ch
			wg.Done()
		}()
		return true
	})

	wg.Wait()
}

func (m *roomMap) Load(id string) (GoroutineSafeRoom, bool) {
	r, ok := m.m.Load(id)
	if !ok {
		return nil, false
	}

	room, _ := r.(GoroutineSafeRoom)
	return room, true
}

func (m *roomMap) LoadOrStore(id string, room GoroutineSafeRoom) (GoroutineSafeRoom, bool) {
	r, loaded := m.m.LoadOrStore(id, room)
	room, _ = r.(GoroutineSafeRoom)
	return room, loaded
}

func (m *roomMap) Range(f func(id string, room GoroutineSafeRoom) bool) {
	m.m.Range(func(key, value interface{}) bool {
		id, _ := key.(string)
		room, _ := value.(GoroutineSafeRoom)
		return f(id, room)
	})
}

func (m *roomMap) Delete(id string) {
	m.m.Delete(id)
}
