package customwidgets

import (
	"sync"
	"sync/atomic"

	"fyne.io/fyne/v2/data/binding"
)

// listerPair holds a data item and its listener.
type listerPair struct {
	data     binding.DataItem
	listener binding.DataListener
}

// binder is a helper for managing data binding.
type binder struct {
	callback atomic.Pointer[func(binding.DataItem)]
	lock     sync.RWMutex
	pair     listerPair // guarded by lock
}

// Bind connects the binder to a data item.
func (b *binder) Bind(data binding.DataItem) {
	listener := binding.NewDataListener(func() {
		f := b.callback.Load()
		if f == nil || *f == nil {
			return
		}
		(*f)(data)
	})
	data.AddListener(listener)
	pair := listerPair{
		data:     data,
		listener: listener,
	}
	b.lock.Lock()
	b.unbindLocked()
	b.pair = pair
	b.lock.Unlock()
}

// CallWithData calls the given function with the bound data item.
func (b *binder) CallWithData(f func(data binding.DataItem)) {
	b.lock.RLock()
	data := b.pair.data
	b.lock.RUnlock()
	f(data)
}

// SetCallback sets the callback function that is called when the data changes.
func (b *binder) SetCallback(f func(data binding.DataItem)) {
	b.callback.Store(&f)
}

// Unbind disconnects the binder from the data item.
func (b *binder) Unbind() {
	b.lock.Lock()
	b.unbindLocked()
	b.lock.Unlock()
}

// unbindLocked disconnects the binder from the data item.
// It is not thread-safe and must be called with the lock held.
func (b *binder) unbindLocked() {
	prev := b.pair
	b.pair = listerPair{nil, nil}
	if prev.listener == nil || prev.data == nil {
		return
	}
	prev.data.RemoveListener(prev.listener)
}
