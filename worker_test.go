package luna

import (
	"testing"
)

type dummyWorker bool

func (v dummyWorker) Start() {}

func (v dummyWorker) Stop() {}

func TestWorkerPool_Get(t *testing.T) {
	worker := NewWorkerPool[string, dummyWorker]()
	const key string = "key"
	const value = dummyWorker(true)
	worker.Add(key, value)

	t.Run("get existing", func(t *testing.T) {
		got := worker.Get(key)
		if got == nil {
			t.Errorf("got %v, want %v", got, key)
		}

		if got.Value != value {
			t.Errorf("got %v, want %v", got.Value, value)
		}

		if got.Key != key {
			t.Errorf("got %v, want %v", got.Key, key)
		}
	})

	t.Run("get non-existing", func(t *testing.T) {
		got := worker.Get("non-existing")
		if got != nil {
			t.Errorf("got %v, want %v", got, nil)
		}
	})

	t.Run("get after delete", func(t *testing.T) {
		worker.Delete(key)
		got := worker.Get(key)
		if got != nil {
			t.Errorf("got %v, want %v", got, nil)
		}
	})
}
