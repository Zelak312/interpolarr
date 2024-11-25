package main

import (
	"context"
	"database/sql"
	"embed"
	"io/fs"
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
		log.Panic("Error when opening sqlite: ", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	if err := pool.PingContext(ctx); err != nil {
		log.Panicf("unable to connect to database: %v", err)
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
		log.Panicf("failed to create fs.FS: %v", err)
	}

	d, err := iofs.New(migrationFs, ".")
	if err != nil {
		log.Panicf("failed to create new instance: %v", err)
	}

	driver, err := sqlite3.WithInstance(s.pool, &sqlite3.Config{})
	if err != nil {
		log.Panic("Error to get driver with instance: ", err)
	}

	m, err := migrate.NewWithInstance("iofs", d, "sqlite3", driver)
	if err != nil {
		log.Panic("Error to make new instance of migration: ", err)
	}

	err = m.Up()
	if err != nil && err.Error() != "no change" {
		log.Panic("Error doing migrations: ", err)
	}
}

func (s *Sqlite) GetVideos() ([]Video, error) {
	querySQL := `SELECT id, path, output_path FROM videos WHERE done = false AND failed = false`
	rows, err := s.pool.Query(querySQL)
	if err != nil {
		return []Video{}, err
	}

	defer rows.Close()
	videos := []Video{}
	for rows.Next() {
		var v Video
		if err := rows.Scan(&v.ID, &v.Path, &v.OutputPath); err != nil {
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
	insertSQL := `INSERT INTO videos (path, output_path, done) VALUES (?, ?, ?)`
	statement, err := s.pool.Prepare(insertSQL)
	if err != nil {
		return 0, err
	}

	defer statement.Close()
	result, err := statement.Exec(video.Path, video.OutputPath, false)
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
	updateSQL := `UPDATE videos SET done = true WHERE id = ?`
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

	return nil
}

func (s *Sqlite) GetVideoRetries(video *Video) (int, error) {
	getRetrySQL := `SELECT retries FROM videos WHERE id = ?`
	statement, err := s.pool.Prepare(getRetrySQL)
	if err != nil {
		return 0, err
	}
	defer statement.Close()

	retries := 0
	err = statement.QueryRow(video.ID).Scan(&retries)
	if err != nil {
		return 0, err
	}

	return retries, nil
}

func (s *Sqlite) UpdateVideoRetries(video *Video, retries int) error {
	updateSQL := `UPDATE videos SET retries = ? WHERE id = ?`
	statement, err := s.pool.Prepare(updateSQL)
	if err != nil {
		return err
	}
	defer statement.Close()

	_, err = statement.Exec(retries, video.ID)
	if err != nil {
		return err
	}

	return nil
}

func (s *Sqlite) FailVideo(video *Video, output string, progErr string) error {
	tx, err := s.pool.Begin()
	if err != nil {
		return nil
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	insertSQL := `INSERT INTO failed_videos (video_id, ffmpeg_output, error) VALUES (?, ?, ?)`
	statement, err := tx.Prepare(insertSQL)
	if err != nil {
		return err
	}

	defer statement.Close()
	_, err = statement.Exec(video.ID, output, progErr)
	if err != nil {
		return err
	}

	markFailedSQL := `UPDATE videos SET failed = ? WHERE id = ?`
	statement, err = s.pool.Prepare(markFailedSQL)
	if err != nil {
		return err
	}
	defer statement.Close()

	_, err = statement.Exec(true, video.ID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Sqlite) DeleteVideoByID(tx *sql.Tx, id int64) error {
	deleteSQL := `DELETE FROM videos WHERE id = ?`
	var statement *sql.Stmt
	var err error
	if tx != nil {
		statement, err = tx.Prepare(deleteSQL)
	} else {
		statement, err = s.pool.Prepare(deleteSQL)
	}

	if err != nil {
		return err
	}

	defer statement.Close()
	_, err = statement.Exec(id)
	return err
}

func (s *Sqlite) GetFailedVideos() ([]FailedVideo, error) {
	querySQL := `SELECT f.id, f.ffmpeg_output, f.error, v.id, v.path, v.output_path FROM failed_videos f
				INNER JOIN videos v ON v.id = f.video_id`
	rows, err := s.pool.Query(querySQL)
	if err != nil {
		return []FailedVideo{}, err
	}

	defer rows.Close()
	videos := []FailedVideo{}
	for rows.Next() {
		var v FailedVideo
		if err := rows.Scan(&v.ID, &v.FFmpegOutput, &v.Error, &v.Video.ID, &v.Video.Path, &v.Video.OutputPath); err != nil {
			return videos, err
		}
		videos = append(videos, v)
	}

	// Check for errors from iterating over rows
	if err := rows.Err(); err != nil {
		return []FailedVideo{}, err
	}

	return videos, nil
}
