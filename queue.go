package main

import (
	"sync"
)

type Queue struct {
	videos []Video
	hub    *Hub
	lock   sync.Mutex
}

func NewQueue(videos []Video, hub *Hub) (Queue, error) {
	return Queue{
		videos: videos,
		hub:    hub,
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
	q.sendUpdate()
}

func (q *Queue) Peek() (Video, bool) {
	q.lock.Lock()
	defer q.lock.Unlock()

	if len(q.videos) == 0 {
		return Video{}, false
	}

	return q.videos[0], true
}

func (q *Queue) Dequeue() (Video, bool) {
	q.lock.Lock()
	defer q.lock.Unlock()

	if len(q.videos) == 0 {
		return Video{}, false
	}

	video := q.videos[0]
	q.videos = q.videos[1:]
	q.sendUpdate()
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
	q.sendUpdate()
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

func (q *Queue) sendUpdate() {
	packet := WsQeueuUpdate{
		WsBaseMessage: WsBaseMessage{
			Type: "queue_update",
		},
		Videos: q.videos,
	}

	q.hub.BroadcastMessage(packet)
}
