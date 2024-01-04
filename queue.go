package main

import (
	"context"
	"database/sql"
	"log"
	"sync"
	"time"

	_ "github.com/glebarez/go-sqlite"
)

var pool *sql.DB

func init() {
	var err error
	pool, err = sql.Open("sqlite", "./interpolarr.db")
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	if err := pool.PingContext(ctx); err != nil {
		log.Fatalf("unable to connect to database: %v", err)
	}

	if err = createTables(); err != nil {
		log.Fatal(err)
	}
}

func createTables() error {
	_, err := pool.Query(`
	CREATE TABLE IF NOT EXISTS video (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		path TEXT NOT NULL,
		output_path TEXT NOT NULL,
		done BOOLEAN
	);	
	`)

	if err != nil {
		return err
	}

	return nil
}

func deleteByID(id int64) error {
	deleteSQL := `DELETE FROM video WHERE id = ?`
	statement, err := pool.Prepare(deleteSQL)
	if err != nil {
		return err
	}

	defer statement.Close()
	_, err = statement.Exec(id)
	return err
}

func markVideoAsDone(videoID int64) error {
	updateSQL := `UPDATE video SET done = true WHERE id = ?`
	statement, err := pool.Prepare(updateSQL)
	if err != nil {
		return err
	}
	defer statement.Close()

	// Execute the statement with the provided video ID
	_, err = statement.Exec(videoID)
	return err
}

type Queue struct {
	items []Video
	lock  sync.Mutex
}

func NewQueue() (Queue, error) {
	querySQL := `SELECT id, path, output_path, done FROM video WHERE done = false`
	rows, err := pool.Query(querySQL)
	if err != nil {
		return Queue{}, err
	}

	defer rows.Close()
	videos := []Video{}
	for rows.Next() {
		var v Video
		if err := rows.Scan(&v.ID, &v.Path, &v.OutputPath, &v.Done); err != nil {
			return Queue{}, err
		}
		videos = append(videos, v)
	}

	// Check for errors from iterating over rows
	if err := rows.Err(); err != nil {
		return Queue{}, err
	}

	return Queue{
		items: videos,
	}, nil
}

func (q *Queue) GetItem() (Video, bool) {
	if len(q.items) == 0 {
		return Video{}, false
	}

	return q.items[0], true
}

func (q *Queue) Enqueue(item Video) error {
	q.lock.Lock()
	defer q.lock.Unlock()

	insertSQL := `INSERT INTO video (path, output_path, done) VALUES (?, ?, ?)`
	statement, err := pool.Prepare(insertSQL)
	if err != nil {
		return err
	}

	defer statement.Close()
	item.Done = false
	result, err := statement.Exec(item.Path, item.OutputPath, item.Done)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	item.ID = id
	q.items = append(q.items, item)

	return nil
}

func (q *Queue) Dequeue() (Video, bool, error) {
	q.lock.Lock()
	defer q.lock.Unlock()

	if len(q.items) == 0 {
		return Video{}, false, nil
	}

	item := q.items[0]
	if err := markVideoAsDone(item.ID); err != nil {
		return Video{}, false, err
	}

	q.items = q.items[1:]
	return item, true, nil
}

func (q *Queue) RemoveByID(id int64) (Video, bool, error) {
	q.lock.Lock()
	defer q.lock.Unlock()

	for i, item := range q.items {
		if item.ID == id {
			if err := deleteByID(item.ID); err != nil {
				return Video{}, false, err
			}

			q.items = append(q.items[:i], q.items[i+1:]...)
			return item, true, nil
		}
	}

	return Video{}, false, nil
}

func (q *Queue) MoveToEnd() {
	q.lock.Lock()
	defer q.lock.Unlock()

	if len(q.items) > 0 {
		item := q.items[0]
		q.items = append(q.items[1:], item)
	}
}
