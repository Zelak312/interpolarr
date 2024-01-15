package main

import (
	"context"
	"database/sql"
	"embed"
	"io/fs"
	"log"
	"time"

	_ "github.com/glebarez/go-sqlite"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

type Sqlite struct {
	pool *sql.DB
}

func NewSqlite(path string) Sqlite {
	// TOOD: may return an error here
	var err error
	pool, err := sql.Open("sqlite", path)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	if err := pool.PingContext(ctx); err != nil {
		log.Fatalf("unable to connect to database: %v", err)
	}

	return Sqlite{
		pool: pool,
	}
}

//go:embed migrations/*.sql
var embedMigrations embed.FS

func (s *Sqlite) RunMigrations() {
	migrationFs, err := fs.Sub(embedMigrations, "migrations")
	if err != nil {
		log.Fatalf("failed to create fs.FS: %v", err)
	}

	d, err := iofs.New(migrationFs, ".")
	if err != nil {
		log.Fatalf("failed to create new instance: %v", err)
	}

	driver, err := sqlite3.WithInstance(s.pool, &sqlite3.Config{})
	if err != nil {
		log.Fatal(err)
	}

	m, err := migrate.NewWithInstance("iofs", d, "sqlite3", driver)
	if err != nil {
		log.Fatal(err)
	}

	err = m.Up()
	if err != nil {
		log.Fatal(err)
	}
}

func (s *Sqlite) GetVideos() ([]Video, error) {
	querySQL := `SELECT id, path, output_path, done FROM video WHERE done = false`
	rows, err := s.pool.Query(querySQL)
	if err != nil {
		return []Video{}, err
	}

	defer rows.Close()
	videos := []Video{}
	for rows.Next() {
		var v Video
		if err := rows.Scan(&v.ID, &v.Path, &v.OutputPath, &v.Done); err != nil {
			return videos, err
		}
		videos = append(videos, v)
	}

	// Check for errors from iterating over rows
	if err := rows.Err(); err != nil {
		return []Video{}, err
	}

	return videos, nil
}

func (s *Sqlite) InsertVideo(video *Video) (int64, error) {
	insertSQL := `INSERT INTO video (path, output_path, done) VALUES (?, ?, ?)`
	statement, err := s.pool.Prepare(insertSQL)
	if err != nil {
		return 0, err
	}

	defer statement.Close()
	video.Done = false
	result, err := statement.Exec(video.Path, video.OutputPath, video.Done)
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	video.ID = id
	return id, nil
}

func (s *Sqlite) MarkVideoAsDone(video *Video) error {
	updateSQL := `UPDATE video SET done = true WHERE id = ?`
	statement, err := s.pool.Prepare(updateSQL)
	if err != nil {
		return err
	}
	defer statement.Close()

	// Execute the statement with the provided video ID
	_, err = statement.Exec(video.ID)
	if err != nil {
		return err
	}

	video.Done = true
	return nil
}

func (s *Sqlite) DeleteVideoByID(id int64) error {
	deleteSQL := `DELETE FROM video WHERE id = ?`
	statement, err := s.pool.Prepare(deleteSQL)
	if err != nil {
		return err
	}

	defer statement.Close()
	_, err = statement.Exec(id)
	return err
}
