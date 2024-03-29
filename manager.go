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
	Negotiate(trans ConnTransport) (NegotiationResult, error)
	GetRoom(id string) (GoroutineSafeRoom, bool)
	GetOrCreateRoom(ctx context.Context, id string) (GoroutineSafeRoom, error)
	NotifyRoomClosed(id string)
	Shutdown()
}

// ErrManagerAlreadyShutdown is returned when access to the manager when that had already shutdown.
var ErrManagerAlreadyShutdown = errors.New("the manger had already shutdown")

type manager struct {
	ctx       context.Context
	cancel    context.CancelFunc
	option    ManagerOption
	wg        *sync.WaitGroup
	closeOnce *sync.Once
	suggester RoomSuggester
	roomMap   roomMap
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
func NewManager(suggester RoomSuggester, option *ManagerOption) Manager {
	if suggester == nil {
		suggester = internalRoomSuggester{}
	}

	if option == nil {
		option = &ManagerOption{}
	}

	if option.Negotiator == nil {
		option.Negotiator = DefaultNegotiator{}
	}

	ctx, cancel := context.WithCancel(context.Background())

	m := &manager{
		ctx:       ctx,
		cancel:    cancel,
		option:    *option,
		wg:        &sync.WaitGroup{},
		closeOnce: &sync.Once{},
		suggester: suggester,
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

func (m *manager) Negotiate(trans ConnTransport) (NegotiationResult, error) {
	return m.option.Negotiator.Negotiate(trans, m)
}

func (m *manager) GetRoom(id string) (GoroutineSafeRoom, bool) {
	return m.roomMap.Load(id)
}

func (m *manager) GetOrCreateRoom(ctx context.Context, id string) (GoroutineSafeRoom, error) {
	m.wg.Add(1)
	defer m.wg.Done()

	select {
	case <-m.ctx.Done():
		return nil, ErrManagerAlreadyShutdown
	default:
	}

	if id != "" {
		r, ok := m.roomMap.Load(id)

		if ok {
			return r, nil
		}
	}

	req := CreateRoomRequest{
		Context: ctx,
		ID:      id,
	}

	suggestion, err := m.suggester.Suggest(req, m)
	if err != nil {
		return nil, err
	}

	if suggestion.RoomHandler == nil {
		panic("suggestion.RoomHandler must not be nil")
	}

	// allow modify the roomID by RoomAllocator.
	id = suggestion.ID

	if id == "" {
		return nil, errors.New("invalid RoomID")
	}

	room := newRoom(id, m, suggestion.RoomHandler, suggestion.Option)
	gsRoom, loaded := m.roomMap.LoadOrStore(id, room.ForGoroutine())

	if !loaded {
		go gsRoom.Run()
	}

	return gsRoom, nil
}

func (m *manager) NotifyRoomClosed(id string) {
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
	m.closeOnce.Do(func() {
		m.cancel()
		// wait finish GetOrCreateRoom methods that has invoked by another goroutine.
		m.wg.Wait()

		wg := sync.WaitGroup{}
		m.roomMap.Range(func(id string, r GoroutineSafeRoom) bool {
			wg.Add(1)
			r.Close()
			go func() {
				<-r.Context().Done()
				wg.Done()
			}()
			return true
		})

		wg.Wait()
	})
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
