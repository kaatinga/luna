package luna_test

import (
	"errors"
	"github.com/kaatinga/luna"
	"testing"
)

type dummyWorker bool

func (v dummyWorker) Start() error {
	return nil
}

func (v dummyWorker) Stop() error {
	return nil
}

type dummyFailingWorker bool

func (v dummyFailingWorker) Start() error {
	return errors.New("dummy start error")
}

func (v dummyFailingWorker) Stop() error {
	return nil
}

type dummyFailingStopWorker bool

func (v dummyFailingStopWorker) Start() error {
	return nil
}

func (v dummyFailingStopWorker) Stop() error {
	return errors.New("dummy stop error")
}

func TestWorkerPool_Get(t *testing.T) {
	w := luna.NewWorkerPool[string, dummyWorker]()
	const key string = "key"
	const value = dummyWorker(true)

	t.Run("add item", func(t *testing.T) {
		err := w.Add(key, value)
		if err != nil {
			t.Errorf("got %v, want %v", err, nil)
		}
	})

	t.Run("get existing", func(t *testing.T) {
		got := w.Get(key)
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
		got := w.Get("non-existing")
		if got != nil {
			t.Errorf("got %v, want %v", got, nil)
		}
	})

	t.Run("get after delete", func(t *testing.T) {
		_ = w.Delete(key)
		got := w.Get(key)
		if got != nil {
			t.Errorf("got %v, want %v", got, nil)
		}
	})

	t.Run("delete one more time", func(t *testing.T) {
		_ = w.Delete(key)
	})

	failingW := luna.NewWorkerPool[string, dummyFailingWorker]()
	failingValue := dummyFailingWorker(true)

	t.Run("worker failed to start", func(t *testing.T) {
		if err := failingW.Add(key, failingValue); err == nil {
			t.Errorf("got %v, want %v", err, "dummy start error")
		}
	})

	t.Run("get failed worker", func(t *testing.T) {
		if got := failingW.Get(key); got != nil {
			t.Errorf("got %v, want %v", got, nil)
		}
	})
}

func TestWorkerPool_Delete(t *testing.T) {
	w := luna.NewWorkerPool[string, dummyWorker]()
	const key string = "key"
	const value = dummyWorker(true)

	if err := w.Add(key, value); err != nil {
		t.Errorf("got %v, want %v", err, nil)
	}

	if err := w.Delete(key); err != nil {
		t.Errorf("got %v, want %v", err, nil)
	}

	failingStopW := luna.NewWorkerPool[string, dummyFailingStopWorker]()
	if err := failingStopW.Add(key, true); err != nil {
		t.Errorf("got %v, want %v", err, nil)
	}

	if err := failingStopW.Delete(key); err == nil || err.Error() != "dummy stop error" {
		t.Errorf("expected 'dummy stop error', got %v", err)
	}
}

func TestWorkerPool_Do(t *testing.T) {
	w := luna.NewWorkerPool[string, dummyWorker]()
	const key string = "key"
	const value = dummyWorker(true)

	if err := w.Add(key, value); err != nil {
		t.Errorf("got %v, want %v", err, nil)
	}

	executed := false
	w.Do(key, func(item *luna.Item[string, dummyWorker]) {
		executed = true
	})

	if !executed {
		t.Errorf("expected function to be executed")
	}

	executed = false
	w.Do("non-existing", func(item *luna.Item[string, dummyWorker]) {
		executed = true
	})

	if executed {
		t.Errorf("function should not be executed for non-existing worker")
	}
}
