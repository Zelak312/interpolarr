package main

import (
	"sync"

	_ "github.com/glebarez/go-sqlite"
)

type Queue struct {
	items []Video
	lock  sync.Mutex
}

func NewQueue(videos []Video) (Queue, error) {
	return Queue{
		items: videos,
	}, nil
}

func (q *Queue) Enqueue(item Video) error {
	q.lock.Lock()
	defer q.lock.Unlock()

	q.items = append(q.items, item)
	return nil
}

func (q *Queue) DequeueItem() (Video, bool) {
	q.lock.Lock()
	defer q.lock.Unlock()

	if len(q.items) == 0 {
		return Video{}, false
	}

	video := q.items[0]
	q.items = q.items[1:]
	return video, true
}

func (q *Queue) DequeueVideoByID(videoID int64) (Video, bool, error) {
	q.lock.Lock()
	defer q.lock.Unlock()

	index := -1
	for i, item := range q.items {
		if item.ID == videoID {
			index = i
			break
		}
	}

	if index == -1 {
		return Video{}, false, nil
	}

	item := q.items[index]
	q.items = append(q.items[:index], q.items[index+1:]...)
	return item, true, nil
}

func (q *Queue) RemoveByID(id int64) (Video, bool, error) {
	q.lock.Lock()
	defer q.lock.Unlock()

	for i, item := range q.items {
		if item.ID == id {
			q.items = append(q.items[:i], q.items[i+1:]...)
			return item, true, nil
		}
	}

	return Video{}, false, nil
}

// func (q *Queue) MoveToEnd() {
// 	q.lock.Lock()
// 	defer q.lock.Unlock()

// 	if len(q.items) > 0 {
// 		item := q.items[0]
// 		q.items = append(q.items[1:], item)
// 	}
// }
