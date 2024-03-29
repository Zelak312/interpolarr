package main

import (
	"sync"
)

type Queue struct {
	videos []Video
	lock   sync.Mutex
}

func NewQueue(videos []Video) (Queue, error) {
	return Queue{
		videos: videos,
	}, nil
}

func (q *Queue) GetVideos() []Video {
	q.lock.Lock()
	defer q.lock.Unlock()

	return q.videos
}

func (q *Queue) Enqueue(item Video) {
	q.lock.Lock()
	defer q.lock.Unlock()

	q.videos = append(q.videos, item)
}

func (q *Queue) Dequeue() (Video, bool) {
	q.lock.Lock()
	defer q.lock.Unlock()

	if len(q.videos) == 0 {
		return Video{}, false
	}

	video := q.videos[0]
	q.videos = q.videos[1:]
	return video, true
}

func (q *Queue) RemoveByID(id int64) (Video, bool) {
	q.lock.Lock()
	defer q.lock.Unlock()

	video, index := q.findByIDInternal(id)
	if index == -1 {
		return Video{}, false
	}

	q.videos = append(q.videos[:index], q.videos[index+1:]...)
	return video, true
}

func (q *Queue) FindByID(id int64) (Video, int) {
	q.lock.Lock()
	defer q.lock.Unlock()

	return q.findByIDInternal(id)
}

func (q *Queue) findByIDInternal(id int64) (Video, int) {
	if len(q.videos) == 0 {
		return Video{}, -1
	}

	index := -1
	for i, item := range q.videos {
		if item.ID == id {
			index = i
			break
		}
	}

	if index == -1 {
		return Video{}, -1
	}

	return q.videos[index], index
}
