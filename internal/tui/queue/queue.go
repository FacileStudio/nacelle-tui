// Package queue manages prompt messages waiting for a run to finish.
package queue

// MaxRows is the maximum number of waiting messages listed individually.
const MaxRows = 3

// Queue manages waiting prompt messages.
type Queue struct {
	items []string
}

// New creates an empty Queue.
func New() *Queue {
	return &Queue{}
}

// Items returns the queued messages slice.
func (q *Queue) Items() []string {
	return q.items
}

// Set replaces the queued messages.
func (q *Queue) Set(items []string) {
	q.items = items
}

// Len returns the count of queued messages.
func (q *Queue) Len() int {
	return len(q.items)
}

// Add appends a message to the queue.
func (q *Queue) Add(text string) {
	q.items = append(q.items, text)
}

// Drop clears all queued messages and returns how many were dropped.
func (q *Queue) Drop() int {
	n := len(q.items)
	q.items = nil
	return n
}

// At returns the message at index.
func (q *Queue) At(i int) string {
	return q.items[i]
}

// PopAt removes and returns the message at index.
func (q *Queue) PopAt(i int) string {
	item := q.items[i]
	q.items = append(q.items[:i], q.items[i+1:]...)
	return item
}
