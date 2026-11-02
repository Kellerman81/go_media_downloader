package apiexternal_v2

import (
	"testing"
)

// fakeNotifier satisfies NotificationProvider through the embedded nil
// interface; the manager only stores and returns providers.
type fakeNotifier struct {
	NotificationProvider
	name string
}

func TestClientManagerNotificationProviders(t *testing.T) {
	cm := NewClientManager()
	a, b := &fakeNotifier{name: "a"}, &fakeNotifier{name: "b"}

	cm.RegisterNotificationProvider("push", a)

	if p, ok := cm.GetNotificationProvider("push"); !ok || p != a {
		t.Errorf("GetNotificationProvider = %v, %t", p, ok)
	}

	if _, err := cm.GetNotificationProviderOrError("missing"); err == nil {
		t.Error("missing provider returned no error")
	}

	if p, err := cm.GetNotificationProviderOrError("push"); err != nil || p != a {
		t.Errorf("GetNotificationProviderOrError = %v, %v", p, err)
	}

	all := cm.GetAllNotificationProviders()
	delete(all, "push")

	if _, ok := cm.GetNotificationProvider("push"); !ok {
		t.Error("GetAllNotificationProviders returned the live map")
	}

	cm.ReplaceNotificationProviders(map[string]NotificationProvider{"mail": b})

	if _, ok := cm.GetNotificationProvider("push"); ok {
		t.Error("replaced set still has the old provider")
	}

	cm.ReplaceNotificationProviders(nil)

	if len(cm.GetAllNotificationProviders()) != 0 {
		t.Error("nil replacement did not clear the set")
	}

	cm.RegisterNotificationProvider("after-nil", a) // must not panic on a nil map
}

func TestGlobalClientManager(t *testing.T) {
	prev, _ := GetGlobalClientManager()
	t.Cleanup(func() { SetGlobalClientManager(prev) })

	SetGlobalClientManager(nil)

	if _, ok := GetGlobalClientManager(); ok {
		t.Error("nil manager reported as set")
	}

	cm := NewClientManager()
	SetGlobalClientManager(cm)

	if got, ok := GetGlobalClientManager(); !ok || got != cm {
		t.Error("global manager not returned")
	}
}
