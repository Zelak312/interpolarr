ALTER TABLE videos
ADD retries SMALLINT DEFAULT 0;
ALTER TABLE videos
ADD failed BOOLEAN DEFAULT 0;
CREATE TABLE failed_videos (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    video_id INTEGER NOT NULL,
    ffmpeg_output TEXT,
    error TEXT NOT NULL,
    FOREIGN KEY (video_id) REFERENCES videos(id)
);