package luna_test

import (
	"sync"
	"testing"

	"github.com/kaatinga/luna"
)

func BenchmarkLuna_Add(b *testing.B) {
	b.StopTimer()
	b.ReportAllocs()
	c := luna.NewWorkerPool[string, testV]()
	var ns = make([]testV, 0, 13888962)
	var name testV
	wg := sync.WaitGroup{}
	b.StartTimer()
	go func() {
		wg.Add(1)
		for i := 0; i < b.N; i++ {
			name = randomUserName()
			ns = append(ns, name)
			for a := 0; a < len(ns); a++ {
				_ = c.Add(string(name), name)
				_ = c.Delete(string(ns[i]))
			}

			_ = c.Add(string(name), name)
		}
		wg.Done()
	}()
	for i := 0; i < len(ns); i++ {
		_ = c.Delete(string(ns[i]))
	}
	wg.Wait()
}

type testV string

func (v testV) Start() error {
	return nil
}

func (v testV) Stop() error {
	return nil
}

//func BenchmarkNoTTLCache_Add(b *testing.B) {
//	b.StopTimer()
//	c := luna.NewWorkerPool[string, testV]()
//	var ns = make([]string, 0, 13888962)
//	var name string
//	b.ReportAllocs()
//	b.StartTimer()
//	for i := 0; i < 1000; i++ {
//		name = randomUserName()
//		ns = append(ns, name)
//		c.Insert(name, testV(name))
//	}
//	for i := 0; i < len(ns); i++ {
//		c.Delete(ns[i])
//	}
//}
