package main

import "sync"

type IDGetter interface {
	GetID() int
}

type Queue[T IDGetter] struct {
	items []T
	lock  sync.Mutex
}

func NewQueue[T IDGetter]() Queue[T] {
	return Queue[T]{
		items: make([]T, 0),
	}
}

func (q *Queue[T]) Enqueue(item T) {
	q.lock.Lock()
	defer q.lock.Unlock()
	q.items = append(q.items, item)
}

func (q *Queue[T]) Dequeue() (T, bool) {
	q.lock.Lock()
	defer q.lock.Unlock()

	if len(q.items) == 0 {
		var zero T // Create a zero value for T
		return zero, false
	}

	item := q.items[0]
	q.items = q.items[1:]
	return item, true
}

func (q *Queue[T]) RemoveByID(id int) (T, bool) {
	q.lock.Lock()
	defer q.lock.Unlock()

	for i, item := range q.items {
		if item.GetID() == id {
			q.items = append(q.items[:i], q.items[i+1:]...)
			return item, true
		}
	}

	var zero T
	return zero, false
}

func (q *Queue[T]) MoveToEnd() {
	q.lock.Lock()
	defer q.lock.Unlock()

	if len(q.items) > 0 {
		item := q.items[0]
		q.items = append(q.items[1:], item)
	}
}
