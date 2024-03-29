# Changelog

All notable changes to this project will be documented in this file.

## [1.0.1-beta] - 2024-01-21

### Bug Fixes

- Not removing from memory queue
- Broken docker container stuff

### Miscellaneous Tasks

- Remove go build on push master

## [1.0.0-beta] - 2024-01-21

### Bug Fixes

- Make slice with NewQueue
- Mark as done instead of delete queued video
- Not taking values from config
- Dequeue video
- Remove model path
- Output folder doesn't exist
- Concurent issue processing same video multiple times
- Taking the filename and not the basepath for creating output folder
- Add sqlite driver import
- Check for context cancelation
- Better fps representation for commands and logs
- Remove inline in config
- Logging right fps when conversion is done
- Ffmpeg commands being wrong
- Possible data race when reading fom queue items
- Migration erroring out when all done
- Add transactions for fail video
- Check if same path before deleting input file
- Remove done from select when getting vids
- Config not getting env correctly
- Workflow for release

### Features

- Setup rotating logger
- Add video to queue endpoint
- Adding list queue endpoint + queue
- Adding delete video queue endpoint
- Add persistance to the queue
- Adding ffmpeg utils
- Adding rife util
- Accelerated gpu nvidia only + fix command
- Add more config
- Queue add db path config
- Process videos
- Adding worker pools
- Adding graceful shutdown
- Add waitgroup for gracefull exit
- Add ffmpeg codec and hardware accel options
- Add minimum fps config
- Add stabilize fps option to config
- Skip videos and change minimum fps to target fps
- Add sql migrations
- Add retry migration
- Fail videos if failed too much
- Copy file when skipping and deleting video
- Add bypass high fps videos
- Adding delete option when interpolation is complet
- Add config setup via env
- Add docker container
- Add README
- Add logPath in config

### Miscellaneous Tasks

- First commit
- Change to nvenc instead for ffmpeg
- .gitignore
- Pass in config instead of multiple args
- Rename config Model to ModelPath
- Change default port to 80
- Refactore queue
- Refactor sqlite
- Refactor poolWorker
- Add ctx to all ffmpeg commands
- Add log for config
- Log ctx error
- Take care of todos
- Add more logs for errors
- Handle video retries error better
- Handle error to keep the program alive
- Change fatal for panics
- Add better error strings
- Add docker readme
- Add github workflows
- Some fixes
- Add release sh script

### Refactor

- Remove done attribute from video struct
- Create process folder after checking the file

<!-- generated by git-cliff -->
