package agentgw

import (
	"testing"
	"time"
)

func TestPendingOfflineCanceledOnReconnect(t *testing.T) {
	prev := offlineGrace
	offlineGrace = 40 * time.Millisecond
	t.Cleanup(func() { offlineGrace = prev })

	s := &service{
		registry:      NewRegistry(),
		offlineTimers: map[string]*time.Timer{},
	}

	s.scheduleOffline("srv-1")
	s.cancelPendingOffline("srv-1")
	time.Sleep(80 * time.Millisecond)

	s.offlineMu.Lock()
	_, pending := s.offlineTimers["srv-1"]
	s.offlineMu.Unlock()
	if pending {
		t.Fatal("canceled offline must leave no pending timer")
	}
}

func TestScheduleOfflineSkippedWhenReconnected(t *testing.T) {
	prev := offlineGrace
	offlineGrace = 40 * time.Millisecond
	t.Cleanup(func() { offlineGrace = prev })

	s := &service{
		registry:      NewRegistry(),
		offlineTimers: map[string]*time.Timer{},
	}
	_ = s.registry.Add("srv-1")

	s.scheduleOffline("srv-1")
	time.Sleep(80 * time.Millisecond)

	if !s.registry.Connected("srv-1") {
		t.Fatal("expected registry to still hold the reconnect")
	}
	s.offlineMu.Lock()
	_, pending := s.offlineTimers["srv-1"]
	s.offlineMu.Unlock()
	if pending {
		t.Fatal("timer should have fired and removed itself without marking offline")
	}
}

func TestScheduleOfflineResetsOnRepeatedDrop(t *testing.T) {
	prev := offlineGrace
	offlineGrace = 60 * time.Millisecond
	t.Cleanup(func() { offlineGrace = prev })

	s := &service{
		registry:      NewRegistry(),
		offlineTimers: map[string]*time.Timer{},
	}

	s.scheduleOffline("srv-1")
	time.Sleep(30 * time.Millisecond)
	s.scheduleOffline("srv-1") // reset — full grace again
	time.Sleep(40 * time.Millisecond)

	s.offlineMu.Lock()
	_, pending := s.offlineTimers["srv-1"]
	s.offlineMu.Unlock()
	if !pending {
		t.Fatal("reset timer should still be pending before grace elapses")
	}

	time.Sleep(40 * time.Millisecond)
	s.offlineMu.Lock()
	_, pending = s.offlineTimers["srv-1"]
	s.offlineMu.Unlock()
	if pending {
		t.Fatal("timer should have elapsed")
	}
}

func TestRegistryConnected(t *testing.T) {
	r := NewRegistry()
	if r.Connected("srv-1") {
		t.Fatal("empty registry must report disconnected")
	}
	c := r.Add("srv-1")
	if !r.Connected("srv-1") {
		t.Fatal("added Conn must report connected")
	}
	if !r.Remove(c) {
		t.Fatal("removing live Conn")
	}
	if r.Connected("srv-1") {
		t.Fatal("removed Conn must report disconnected")
	}
}
