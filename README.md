# Interpolarr

Interpolarr is an innovative tool designed to upscale videos to a higher frame rate using [RIFE NCNN Vulkan](https://github.com/TNTwise/rife-ncnn-vulkan).

If you want to support my work<br>
[!["Buy Me A Coffee"](https://www.buymeacoffee.com/assets/img/custom_images/orange_img.png)](https://www.buymeacoffee.com/zelak)

## Table of Contents

1. [Features](#features)
2. [How it Works](#how-it-works)
3. [Configuration](#configuration)
    - [Env Variables](#env-variables-can-also-be-used)
    - [Configuration Notes](#configuration-notes)
4. [Configuration with Docker](#configuration-with-docker)
    - [Unchangable Docker Configurations](#unchangable-docker-configurations)
    - [Default Docker Configurations](#default-docker-configurations)
5. [API Endpoints](#api-endpoints)
    - [Video Queue Structure](#video-queue-structure)
6. [Usage](#usage)
7. [Contributing](#contributing)
8. [License](#license)

## Features

-   **High FPS Conversion**: Convert videos to a higher frame per second rate effectively.
-   **Flexible Configuration**: Customize settings through a comprehensive YAML configuration file.
-   **API Integration**: Utilize a straightforward API for queue management and video processing.
-   **RIFE NCNN Vulkan Technology**: Leverage the latest advancements in frame interpolation technology.
-   **24/7 Video Interpolation**: Aimed to not crash nor exit without explicitly asking it to, for less downtime as possible

## How it works

Interpolarr will process video in a queue format, first in first out. It will extract the audio, the video frames will be extracted. Those frames will be interpolated with rife to the desired frame rate and then the video will be reconstructed with the audio and the new framerate.

## Configuration

Interpolarr is configured using a YAML file. Below is the structure of the configuration file with default values:

**Values that are in between <> are required**
**Values that are in between [] are optional**

```yaml
---
bindAddress: "127.0.0.1"
port: 80
rifeBinary: <path_to_rife_binary>
processFolder: <path_to_process_folder>
databasePath: <path_to_database>
logPath: "./logs"
modelPath: "rife-v4.7"
workers: 1
targetFPS: 60.0
deleteInputFileWhenFinished: false
deleteOutputIfAlreadyExist: false
CopyFileToDestinationOnSkip: false
ffmpegOptions:
    HWAccelDecodeFlag: [decode_flag]
    HWAccelEncodeFlag: [encode_flag]
```

### Env variables can also be used

Interpolarr will use the env if specified over the config file, the envs variable are the name names but different format, example `bindAddress` would be `BIND_ADDRESS` for env, so you need to make sure you convert the name correctly. For ffmpeg options, a nested struct, it will be the same but with the struct prefix `FFMPEG_OPTIONS_VIDEO_CODEC`

### Configuration notes

-   `bindAddress`: if set to `127.0.0.1` will only listen locally
-   `rifeBinary`: path to the rife binary
-   `processFolder`: path to a temporary process folder, video frames and other things for interpolation will be temporary saved there
-   `databasePath`: path to where the database will be stored example: `./interpolarr.db`
-   `logPath`: path to where the log files will be stored, should be a folder
-   `modelPath`: path to which rife model should be used. The default path of `rife-v4.7` means that the folder should be where interpolarr is executed, **it is a path**
-   `workers`: how many videos can be interpolated concurrently, **using 1 is highly recommended unless you know your gpu or cpu can handle more**
-   `targetFPS`: Which FPS should the videos be after interpoaltion
-   `deleteInputFileWhenFinished`: When the interpolation of the video is done, interpolarr will delete the input file, **be careful with this if you don't want to lose the input (orignal) file, use at your own risk**
-   `deleteOutputIfAlreadyExist`: If the output file already exist (output being the converted file), it will delete that file if true and continue the process for the conversion. If it is false, it will skip the this file
-   `CopyFileToDestinationOnSkip`: When a file is skipped, it's because it already is at the target FPS or higher, this option will copy the file to the output if the file is skipped

## Configuration with docker

I personally recommend the use of docker compose so I made a [compose.yml](compose.yml) file to show how to use the docker container<br>
The environment variables from passed to docker will override a config yml file if mounted, the same ENV vars from the configuration section can be used and passed to docker. There [unchangable docker configuration](#unchangable-docker-configurations) and there are also [default docker configurations](#default-docker-configurations), these are important to read

### Unchangable docker configurations

Some of the configuration are unchangable in docker for simple reasons like, the rife binary being downloaded to a certain path and I don't want people to accidently change this configuration and have their container suddendly break. To view those unchangable configuration, they are in [.env.docker](.env.docker)

### Default docker configurations

Those are default docker configurations, they are overridable from a mount config.yml file and also the ENV variables. These are what most people would probably use when using docker, they are in stored in [docker_default.yml](docker_default.yml)

## API Endpoints

-   **GET `/ping`**: Returns a simple `{"message": "ping"}` response for health check.
-   **GET `/queue`**: Lists the current video processing queue.
-   **POST `/queue`**: Adds a video to the processing queue. Returns a 200 status on success.
-   **DELETE `/queue/:id`**: Removes a video from the queue based on its ID.

### Video Queue Structure

GET /queue will return a list of this structure

POST /queue takes this structure as a json body, id should not be sent on this endpoint, it will give an id to the video automatically

```json
{
    "id": "<video_id>",
    "path": "<path_to_video>",
    "outPath": "<output_path>"
}
```

## Usage

To use Interpolarr, follow these steps:

1. **Set Up Configuration**: Create a YAML configuration file using the structure provided above. Customize the settings as per your requirements.
2. **Start the Server**: Run Interpolarr with the command line argument `--config_path` to specify the path of your configuration file. If the config file is in the same path as where interpolarr is exectued, it is not needed if the file is named `config.yml`
3. **API Interaction**: Use the API endpoints to manage the video processing queue.

## Contributing

We welcome contributions to Interpolarr! If you have suggestions or improvements, please open an issue in the repository before hand then submit a pull request if approved.

## License

Interpolarr is released under MIT. For more details, see the [LICENSE](LICENSE) file in the repository.

---

Enjoy enhanced video experiences with Interpolarr!
