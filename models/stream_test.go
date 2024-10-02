package models_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/onflow/flow-evm-gateway/models"
	"github.com/stretchr/testify/require"
)

func Test_Stream(t *testing.T) {

	t.Run("unsubscribe before subscribing", func(t *testing.T) {
		p := newMockPublisher()
		s := newMockSubscription()

		require.NotPanics(t, func() {
			p.Unsubscribe(s)
		})
	})

	t.Run("subscribe, publish, unsubscribe, publish", func(t *testing.T) {
		p := newMockPublisher()
		s1 := newMockSubscription()
		s2 := newMockSubscription()

		p.Subscribe(s1)
		p.Subscribe(s2)

		p.Publish(mockData{})

		require.Equal(t, uint(1), s1.callCount)
		require.Equal(t, uint(1), s2.callCount)

		p.Unsubscribe(s1)

		p.Publish(mockData{})

		require.Equal(t, uint(1), s1.callCount)
		require.Equal(t, uint(2), s2.callCount)
	})

	t.Run("concurrent subscribe, publish, unsubscribe, publish", func(t *testing.T) {

		p := newMockPublisher()

		stopPublishing := make(chan struct{})

		// publishing
		go func() {
			for {
				select {
				case <-stopPublishing:
					return
				case <-time.After(time.Millisecond * 1):
					p.Publish(mockData{})
				}
			}
		}()

		wg := sync.WaitGroup{}

		// 10 goroutines adding 10 subscribers each
		// waiting for 100 ms to make sure all goroutines are added
		// and then unsubscribe all
		wg.Add(10)
		for i := 0; i < 10; i++ {
			go func() {
				defer wg.Done()

				subscriptions := make([]*mockSubscription, 10)

				for j := 0; j < 10; j++ {
					s := newMockSubscription()
					subscriptions[j] = s
					p.Subscribe(s)
				}

				<-time.After(time.Millisecond * 100)

				for _, s := range subscriptions {
					p.Unsubscribe(s)
				}

				// there should be at least 50 calls
				for j := 0; j < 10; j++ {
					require.Greater(t, subscriptions[j].callCount, uint(50))
				}
			}()
		}

		wg.Wait()
		close(stopPublishing)
	})

	t.Run("error handling", func(t *testing.T) {
		p := newMockPublisher()
		s := &mockSubscription{}
		errContent := fmt.Errorf("failed to process data")

		s.Subscription = models.NewSubscription[mockData](func(data mockData) error {
			s.callCount++
			return errContent
		})

		p.Subscribe(s)

		go func() {
			select {
			case err := <-s.Error():
				require.ErrorIs(t, err, errContent)
			case <-time.After(time.Millisecond * 10):
				require.Fail(t, "should have received error")
			}
		}()

		// wait for the goroutine to subscribe to error channel
		<-time.After(time.Millisecond * 1)

		p.Publish(mockData{})
	})
}

type mockData struct{}

type mockSubscription struct {
	*models.Subscription[mockData]
	callCount uint
}

func newMockSubscription() *mockSubscription {
	s := &mockSubscription{}
	s.Subscription = models.NewSubscription[mockData](func(data mockData) error {
		s.callCount++
		return nil
	})
	return s
}

func newMockPublisher() *models.Publisher[mockData] {
	return models.NewPublisher[mockData]()
}
