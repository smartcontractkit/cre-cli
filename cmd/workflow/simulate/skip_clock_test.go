package simulate

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSkipClock_SkipFiresPendingTimersOnce(t *testing.T) {
	t.Parallel()

	clock := NewSkipClock()
	var fired atomic.Int32
	clock.AfterFunc(200*time.Millisecond, func() { fired.Add(1) })

	clock.Skip()
	require.Eventually(t, func() bool { return fired.Load() == 1 }, time.Second, 10*time.Millisecond)

	// The real timer was stopped, so it does not fire again when due.
	time.Sleep(400 * time.Millisecond)
	assert.Equal(t, int32(1), fired.Load())
}

func TestSkipClock_TimersFireWhenDue(t *testing.T) {
	t.Parallel()

	clock := NewSkipClock()
	var fired atomic.Int32
	clock.AfterFunc(10*time.Millisecond, func() { fired.Add(1) })
	require.Eventually(t, func() bool { return fired.Load() == 1 }, time.Second, 10*time.Millisecond)

	// A fired timer is no longer pending, so skipping does not run it again.
	clock.Skip()
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(1), fired.Load())
}

func TestSkipClock_StoppedTimersDoNotFire(t *testing.T) {
	t.Parallel()

	clock := NewSkipClock()
	var fired atomic.Int32
	timer := clock.AfterFunc(time.Hour, func() { fired.Add(1) })
	timer.Stop()

	clock.Skip()
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(0), fired.Load())
}
