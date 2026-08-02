package eventbus

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func quietRun(t *testing.T, e *EventBus) (stop func()) {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})

	go func() {
		_ = e.Run(ctx)

		close(done)
	}()

	return func() {
		cancel()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("eventbus.Run did not return within 2s of context cancellation")
		}
	}
}

func newRunningBus(t *testing.T) *EventBus {
	t.Helper()

	e := New()
	stop := quietRun(t, e)
	t.Cleanup(stop)

	return e
}

type recv struct {
	mu   sync.Mutex
	seen []any
}

func (r *recv) cb(data any) {
	r.mu.Lock()
	r.seen = append(r.seen, data)
	r.mu.Unlock()
}

func (r *recv) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return len(r.seen)
}

func (r *recv) snapshot() []any {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]any, len(r.seen))
	copy(out, r.seen)

	return out
}

func eventually(t *testing.T, cond func() bool, msgAndArgs ...any) {
	t.Helper()
	eventuallyFor(t, time.Second, cond, msgAndArgs...)
}

func eventuallyFor(t *testing.T, waitFor time.Duration, cond func() bool, msgAndArgs ...any) {
	t.Helper()
	require.Eventually(t, cond, waitFor, 2*time.Millisecond, msgAndArgs...)
}

func TestSubscribe_PanicsOnUnknownEvent(t *testing.T) {
	t.Parallel()

	e := New()

	const unknown Event = "does.not.exist"

	e.mu.RLock()
	_, ok := e.subscribers[unknown]
	e.mu.RUnlock()
	assert.False(t, ok)

	assert.Panics(t, func() {
		e.Subscribe(unknown, func(any) {})
	})
}

func TestPublish_BeforeRunIsBufferedAndDoesNotBlock(t *testing.T) {
	t.Parallel()

	e := New()

	done := make(chan struct{})
	go func() {
		defer close(done)

		for range 50 {
			e.Publish(EventTagMutation, GroupMutationEvent{GID: sampleGID()})
		}
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Publish before Run blocked; expected buffered send to return immediately")
	}
}

func TestRun_DeliversPublishedEventToSubscriber(t *testing.T) {
	t.Parallel()

	e := newRunningBus(t)

	r := &recv{}
	e.Subscribe(EventTagMutation, r.cb)

	want := GroupMutationEvent{GID: sampleGID()}
	e.Publish(EventTagMutation, want)

	eventually(t, func() bool { return r.count() == 1 }, "subscriber should receive one event")

	got := r.snapshot()
	require.Len(t, got, 1)
	assert.Equal(t, want, got[0], "delivered data should match what was published")
}

func TestRun_DeliversToMultipleSubscribersOfSameEvent(t *testing.T) {
	t.Parallel()

	e := newRunningBus(t)

	r1, r2, r3 := &recv{}, &recv{}, &recv{}
	e.Subscribe(EventEntityMutation, r1.cb)
	e.Subscribe(EventEntityMutation, r2.cb)
	e.Subscribe(EventEntityMutation, r3.cb)

	want := GroupMutationEvent{GID: sampleGID()}
	e.Publish(EventEntityMutation, want)

	eventually(t, func() bool { return r1.count()+r2.count()+r3.count() == 3 },
		"all three subscribers should each receive the event")

	for _, r := range []*recv{r1, r2, r3} {
		require.Len(t, r.snapshot(), 1)
		assert.Equal(t, want, r.snapshot()[0])
	}
}

func TestRun_DeliversAcrossAllEventTypes(t *testing.T) {
	t.Parallel()

	e := newRunningBus(t)

	handlers := map[Event]*recv{}

	for _, ev := range []Event{EventTagMutation, EventEntityMutation, EventUserMutation, EventExportMutation, EventImportMutation} {
		r := &recv{}
		handlers[ev] = r
		e.Subscribe(ev, r.cb)
	}

	order := []Event{EventImportMutation, EventTagMutation, EventUserMutation, EventExportMutation, EventEntityMutation}
	for _, ev := range order {
		e.Publish(ev, GroupMutationEvent{GID: sampleGID()})
	}

	for _, ev := range order {
		eventually(t, func() bool { return handlers[ev].count() == 1 },
			"subscriber for %s should receive exactly one event", ev)
	}
}

func TestRun_DoesNotDeliverToSubscribersOfOtherEvents(t *testing.T) {
	t.Parallel()

	e := newRunningBus(t)

	tag := &recv{}
	e.Subscribe(EventTagMutation, tag.cb)

	entity := &recv{}
	e.Subscribe(EventEntityMutation, entity.cb)

	e.Publish(EventTagMutation, GroupMutationEvent{GID: sampleGID()})

	eventually(t, func() bool { return tag.count() == 1 }, "tag subscriber should receive its event")
	assert.Zero(t, entity.count(), "entity subscriber must not receive a tag event")
}

func TestRun_PreservesPublishOrderPerEvent(t *testing.T) {
	t.Parallel()

	e := newRunningBus(t)

	var mu sync.Mutex

	got := []int{}

	e.Subscribe(EventTagMutation, func(data any) {
		mu.Lock()
		if n, ok := data.(int); ok {
			got = append(got, n)
		}
		mu.Unlock()
	})

	const n = 30
	for i := range n {
		e.Publish(EventTagMutation, i)
	}

	eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()

		return len(got) == n
	}, "all %d events should be delivered in order", n)

	mu.Lock()
	defer mu.Unlock()

	want := make([]int, n)
	for i := range n {
		want[i] = i
	}

	assert.Equal(t, want, got, "events for a single subscriber must be delivered in publish order")
}

func TestRun_ConcurrentPublishersAreSafe(t *testing.T) {
	t.Parallel()
	e := newRunningBus(t)

	var delivered atomic.Int64

	e.Subscribe(EventEntityMutation, func(any) { delivered.Add(1) })

	const (
		publishers   = 8
		perPublisher = 50
	)

	var wg sync.WaitGroup
	wg.Add(publishers)

	for range publishers {
		go func() {
			defer wg.Done()

			for range perPublisher {
				e.Publish(EventEntityMutation, GroupMutationEvent{GID: sampleGID()})
			}
		}()
	}

	wg.Wait()

	want := int64(publishers * perPublisher)
	eventuallyFor(t, 2*time.Second, func() bool { return delivered.Load() == want },
		"all %d concurrently-published events should be delivered", want)
	assert.Equal(t, want, delivered.Load())
}

func TestRun_ConcurrentSubscribeAndPublishAreSafe(t *testing.T) {
	t.Parallel()
	e := newRunningBus(t)

	var delivered atomic.Int64

	e.Subscribe(EventEntityMutation, func(any) { delivered.Add(1) })

	const subscribers = 10

	var wg sync.WaitGroup
	wg.Add(subscribers + 1)

	go func() {
		defer wg.Done()

		for range 200 {
			e.Publish(EventEntityMutation, nil)
		}
	}()

	for range subscribers {
		go func() {
			defer wg.Done()

			e.Subscribe(EventEntityMutation, func(any) {})
		}()
	}

	wg.Wait()

	eventuallyFor(t, 2*time.Second, func() bool { return delivered.Load() == 200 },
		"all events published during concurrent Subscribe should be delivered to the stable subscriber")
}

func TestRun_ReturnsNilOnContextCancellation(t *testing.T) {
	t.Parallel()

	e := New()

	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan error, 1)
	go func() { done <- e.Run(ctx) }()

	cancel()

	select {
	case err := <-done:
		require.NoError(t, err, "Run should return nil when its context is cancelled")
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return within 2s of context cancellation")
	}
}

func TestRun_CancellationStopsDelivery(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		e := New()

		r := &recv{}
		e.Subscribe(EventTagMutation, r.cb)

		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan struct{})

		go func() {
			_ = e.Run(ctx)

			close(done)
		}()

		cancel()
		<-done

		before := r.count()

		e.Publish(EventTagMutation, GroupMutationEvent{GID: sampleGID()})
		synctest.Wait()
		assert.Equal(t, before, r.count(), "no delivery should occur after Run has stopped")
	})
}

func TestRun_ConcurrentStartAllowsOneRunner(t *testing.T) {
	t.Parallel()

	e := New()
	ctx, cancel := context.WithCancel(t.Context())
	start := make(chan struct{})
	results := make(chan any, 2)

	for range 2 {
		go func() {
			<-start

			defer func() {
				results <- recover()
			}()

			_ = e.Run(ctx)
		}()
	}

	close(start)

	var first any
	select {
	case first = <-results:
	case <-time.After(2 * time.Second):
		t.Fatal("neither concurrent Run call completed")
	}

	require.NotNil(t, first, "one concurrent Run call must reject the duplicate start")

	cancel()

	select {
	case second := <-results:
		assert.Nil(t, second, "the winning Run call must stop normally after cancellation")
	case <-time.After(2 * time.Second):
		t.Fatal("winning Run call did not stop after cancellation")
	}
}

func TestRun_PanickingSubscriberDoesNotStopDelivery(t *testing.T) {
	t.Parallel()

	e := newRunningBus(t)

	panicRan := atomic.Int32{}

	e.Subscribe(EventTagMutation, func(any) {
		panicRan.Add(1)
		panic("boom")
	})

	second := &recv{}
	e.Subscribe(EventTagMutation, second.cb)

	e.Publish(EventTagMutation, GroupMutationEvent{GID: sampleGID()})

	eventually(t, func() bool {
		return panicRan.Load() == 1 && second.count() == 1
	}, "a panicking subscriber must not block later subscribers")

	e.Publish(EventTagMutation, GroupMutationEvent{GID: sampleGID()})

	eventually(t, func() bool {
		return panicRan.Load() == 2 && second.count() == 2
	}, "the event loop must continue after a subscriber panic")
}

func TestRun_MultipleBusesAreIndependent(t *testing.T) {
	t.Parallel()
	e1 := newRunningBus(t)
	e2 := newRunningBus(t)

	r1, r2 := &recv{}, &recv{}
	e1.Subscribe(EventTagMutation, r1.cb)
	e2.Subscribe(EventTagMutation, r2.cb)

	e1.Publish(EventTagMutation, GroupMutationEvent{GID: sampleGID()})

	eventually(t, func() bool { return r1.count() == 1 }, "only bus 1's subscriber should see bus 1's event")
	assert.Zero(t, r2.count(), "bus 2 must not receive events published to bus 1")
}

func TestPublish_AnyDataIsPassedThroughUntouched(t *testing.T) {
	t.Parallel()

	e := newRunningBus(t)

	cases := []struct {
		name string
		data any
	}{
		{"group mutation", GroupMutationEvent{GID: sampleGID()}},
		{"nil", nil},
		{"string", "hello"},
		{"int", 42},
		{"slice", []int{1, 2, 3}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &recv{}
			e.Subscribe(EventUserMutation, r.cb)

			e.Publish(EventUserMutation, tc.data)

			eventually(t, func() bool { return r.count() == 1 }, "subscriber should receive the published data")

			got := r.snapshot()
			require.Len(t, got, 1)
			assert.Equal(t, tc.data, got[0])
		})
	}
}

func TestRun_PublishesUnblockAfterBufferDrains(t *testing.T) {
	t.Parallel()

	e := newRunningBus(t)

	block := make(chan struct{})

	var processed atomic.Int32

	e.Subscribe(EventExportMutation, func(any) {
		processed.Add(1)
		// Block delivery so the event channel reaches capacity.
		<-block
	})

	// `Run` holds one event in the subscriber while the channel buffers `100`.
	// The `102nd` publish must block until the subscriber is released.
	const total = 102

	var published atomic.Bool

	var wg sync.WaitGroup

	wg.Go(func() {
		for range total {
			e.Publish(EventExportMutation, nil)
		}

		published.Store(true)
	})

	eventually(t, func() bool { return processed.Load() == 1 }, "first event should reach the blocking subscriber")
	require.False(t, published.Load(), "publisher must stay blocked on the full buffer until the subscriber is released")

	close(block)
	wg.Wait()

	assert.True(t, published.Load(), "publisher should unblock once the buffer drains")
	eventually(t, func() bool { return processed.Load() == total }, "all events should be processed once unblocked")
}

func sampleGID() uuid.UUID {
	return uuid.UUID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10}
}
