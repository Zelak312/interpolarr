ALTER TABLE video
ADD retries SMALLINT DEFAULT 0;
CREATE TABLE failed_videos (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    video_id INTEGER NOT NULL,
    ffmpeg_output TEXT,
    error TEXT NOT NULL
);