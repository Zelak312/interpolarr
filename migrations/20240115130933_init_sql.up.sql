CREATE TABLE video (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    path TEXT NOT NULL,
    output_path TEXT NOT NULL,
    done BOOLEAN
);