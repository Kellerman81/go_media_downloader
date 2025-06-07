package pool

import (
	"sync"
	"testing"
	"time"
)

func TestPoolObjGet(t *testing.T) {
	p := NewPool[string](5, 0, func(s *string) { *s = "initialized" }, nil)

	obj := p.Get()
	if *obj != "initialized" {
		t.Errorf("Expected initialized object, got %v", *obj)
	}

	for i := 0; i < 3; i++ {
		p.Put(obj)
		obj2 := p.Get()
		if obj != obj2 {
			t.Error("Expected same object reference from pool")
		}
	}
}

func TestPoolObjPut(t *testing.T) {
	destructorCalled := 0
	p := NewPool[int](3, 0, nil, func(i *int) bool {
		destructorCalled++
		return false
	})

	obj1 := p.Get()
	obj2 := p.Get()
	obj3 := p.Get()

	p.Put(obj1)
	p.Put(obj2)
	p.Put(obj3)

	if len(p.objs) != 3 {
		t.Errorf("Expected pool size 3, got %d", len(p.objs))
	}

	if destructorCalled != 3 {
		t.Errorf("Expected destructor called 3 times, got %d", destructorCalled)
	}
}

func TestPoolObjInit(t *testing.T) {
	var p Poolobj[int]
	constructorCalled := 0

	p.Init(3, func(i *int) {
		constructorCalled++
		*i = constructorCalled
	}, nil)

	if constructorCalled != 3 {
		t.Errorf("Expected constructor called 3 times, got %d", constructorCalled)
	}

	if len(p.objs) != 3 {
		t.Errorf("Expected 3 objects in pool, got %d", len(p.objs))
	}
}

func TestSizedWaitGroupConcurrent(t *testing.T) {
	swg := NewSizedGroup(2)
	results := make([]int, 0, 4)
	var mu sync.Mutex

	for i := 0; i < 4; i++ {
		i := i
		go func() {
			swg.Add()
			defer swg.Done()

			time.Sleep(10 * time.Millisecond)
			mu.Lock()
			results = append(results, i)
			mu.Unlock()
		}()
	}

	swg.Wait()

	if len(results) != 4 {
		t.Errorf("Expected 4 results, got %d", len(results))
	}
}

func TestSizedWaitGroupZeroLimit(t *testing.T) {
	swg := NewSizedGroup(0)
	if swg.Size != 1 {
		t.Errorf("Expected size 1 for zero limit, got %d", swg.Size)
	}
}

func TestPoolObjDestructorTrue(t *testing.T) {
	p := NewPool[string](5, 0, nil, func(s *string) bool {
		return true
	})

	obj := p.Get()
	p.Put(obj)

	if len(p.objs) != 0 {
		t.Errorf("Expected empty pool when destructor returns true, got size %d", len(p.objs))
	}
}

func TestPoolObjNilPut(t *testing.T) {
	p := NewPool[string](5, 0, nil, nil)
	p.Put(nil)

	if len(p.objs) != 0 {
		t.Error("Expected pool to ignore nil objects")
	}
}
