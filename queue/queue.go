package queue

import (
	"errors"
	"sync"
)

var (
	ErrEmptyQueue   = errors.New("queue is empty")
	ErrItemNotFound = errors.New("item never pushed to queue")
)

// bool in map represents whether the item should be processed
// and the presence of item in map represents that a crawler have seen this URL before
// i.e. the URL is not unique

// UniqueQueue holds unique values in its queue
type UniqueQueue struct {
	queue  []string
	strMap map[string]bool
	mu     sync.Mutex
}

// NewQueue returns a pointer to new UniqueQueue
func NewQueue() *UniqueQueue {
	return &UniqueQueue{
		strMap: map[string]bool{},
		mu:     sync.Mutex{},
	}
}

// Size returns the size of queue
func (q *UniqueQueue) Size() int {
	return len(q.queue)
}

// IsEmpty tells if queue is empty or not
func (q *UniqueQueue) IsEmpty() bool {
	return q.Size() == 0
}

// FirstEncounter returns true if the item was seen for the first time by the queue in its lifetime
func (q *UniqueQueue) FirstEncounter(item string) bool {
	if _, ok := q.strMap[item]; ok {
		// if item present in map return false, item seen before
		return false
	}
	// new item return true
	return true
}

// GetMapValue returns value of key inside the map used by
// UniqueQueue.
//
// It throws ErrItemNotFound when the item couldn't
// be found in the map, this means that the item was never
// pushed to the queue.
//
// Thread safe.
func (q *UniqueQueue) GetMapValue(key string) (bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if v, ok := q.strMap[key]; ok {
		return v, nil
	}
	return false, ErrItemNotFound
}

// Because the crawler will be processing items individually
// and will update the key value in map seperately for each item,
// mutual exclusion is not required in SetMapValue method.

// SetMapValue set value of key inside the map used by
// UniqueQueue.
func (q *UniqueQueue) SetMapValue(key string, value bool) {
	// q.mu.Lock()
	// defer q.mu.Unlock()
	q.strMap[key] = value
}

// Push appends item to the queue after checking that item was
// never seen before by the queue. Returns true if item was added to the queue.
//
// NOP when item was seen earlier in queue's lifetime.
// Default item value is 'false'.
//
// Thread safe.
func (q *UniqueQueue) Push(item string) (success bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.FirstEncounter(item) {
		q.strMap[item] = false
		q.queue = append(q.queue, item)
		success = true
	}
	return
}

// PushForce appends item to the queue WITHOUT checking that the item was seen before.
// Useful when item was not processed successfully and needs to be reprocessed.
//
// Default item value is 'false'.
//
// Thread safe.
func (q *UniqueQueue) PushForce(item string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.strMap[item] = false
	q.queue = append(q.queue, item)
}

// Pop pops first item from the queue.
// Returns ErrEmptyQueue when empty.
//
// Thread safe.
func (q *UniqueQueue) Pop() (string, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.IsEmpty() {
		var x string
		x, q.queue = q.queue[0], q.queue[1:]
		return x, nil
	}
	return "", ErrEmptyQueue
}