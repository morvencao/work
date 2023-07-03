package watcher

import (
	"sync"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog/v2"
)

type Receiver interface {
	watch.Interface
	Receive(evt watch.Event)
}

// MessageQueueWatcher turns any stream for which you can write a Decoder interface
// into a watch.Interface.
type MessageQueueWatcher struct {
	sync.Mutex

	result chan watch.Event
	done   chan struct{}
}

// NewMessageQueueWatcher creates a MessageQueueWatcher from the given configurations.
func NewMessageQueueWatcher() *MessageQueueWatcher {
	mw := &MessageQueueWatcher{
		// It's easy for a consumer to add buffering via an extra
		// goroutine/channel, but impossible for them to remove it,
		// so nonbuffered is better.
		result: make(chan watch.Event),
		// If the watcher is externally stopped there is no receiver anymore
		// and the send operations on the result channel, especially the
		// error reporting might block forever.
		// Therefore a dedicated stop channel is used to resolve this blocking.
		done: make(chan struct{}),
	}

	// go sw.receive()
	return mw
}

// ResultChan implements Interface.
func (mw *MessageQueueWatcher) ResultChan() <-chan watch.Event {
	return mw.result
}

// Stop implements Interface.
func (mw *MessageQueueWatcher) Stop() {
	// Call Close() exactly once by locking and setting a flag.
	mw.Lock()
	defer mw.Unlock()
	// closing a closed channel always panics, therefore check before closing
	select {
	case <-mw.done:
		close(mw.result)
	default:
		close(mw.done)
	}
}

// receive reads result from the decoder in a loop and sends down the result channel.
func (mw *MessageQueueWatcher) Receive(evt watch.Event) {
	obj, _ := meta.Accessor(evt.Object)
	klog.Infof("receive the event %v for %v", evt.Type, obj.GetName())

	mw.result <- evt
}
