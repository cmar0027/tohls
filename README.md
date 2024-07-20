# tohls

Convert video files into varibale quality hls streams.

## Installation
- Make sure you have ffmpeg and ffprobe already installed
- Make sure you have go tools installed
- `go install github.com/cmar0027/tohls`

## Example
Pretend you have a video file myvideo.mp4

1. Create new directory

   `> mkdir hls && cd hls`

2. Convert into hls

    `> tohls -f 360p:: -f 480p:: -f 720p:: -f 1080p::  ../myvideo.mp4`

3. The current directory will contain the main playlist, along with subplaylists and video files:
    
    ```
    .
    ├── myvideo.mp4.master.m3u8
    ├── v1280x720.m3u8
    ├── v1280x720_000.ts
    ├── v1280x720_001.ts
    ├── v1280x720_002.ts
    ├── v1280x720_003.ts
    ├── v1280x720_004.ts
    ├── v1280x720_005.ts
    ├── v1920x1080.m3u8
    ├── v1920x1080_000.ts
    ├── v1920x1080_001.ts
    ├── v1920x1080_002.ts
    ├── v1920x1080_003.ts
    ├── v1920x1080_004.ts
    ├── v1920x1080_005.ts
    ├── v640x360.m3u8
    ├── v640x360_000.ts
    ├── v640x360_001.ts
    ├── v640x360_002.ts
    ├── v640x360_003.ts
    ├── v640x360_004.ts
    ├── v640x360_005.ts
    ├── v854x480.m3u8
    ├── v854x480_000.ts
    ├── v854x480_001.ts
    ├── v854x480_002.ts
    ├── v854x480_003.ts
    ├── v854x480_004.ts
    └── v854x480_005.ts
    ```

4. Upload wherever you want
    `aws s3 sync . s3://mybucket/myvideo`


## Usage

```
tohls OPTIONS FILES
	
OPTIONS

-f FORMAT       Specify formats into which input files will be converted.
                To specify multiple formats, the -f option must be repeated
                multiple times.
                Each format value must be a properly formatted string according to the 
                following grammar (upper case words denote non terminals, 
                lower case characters denote terminals, '=' defines non terminals, '|' 
                is used to specify multiple options when defining a non terminal, '<' 
                and '>' used to give an english descriptioni of the value):
                
                    FORMAT = SIZE:FRAMERATE:QUALITY

                    SIZE = WxH | Hp
                    W = <video width in pixels>
                    H = <video height in pixels>
                	
                    FRAMERATE = <framerate of the video, can be a floating point number> | EMPTY

                    QUALITY = <quality factor of the video, usually between 0.07 and 0.15> | EMPTY

                    EMPTY = <empty string>
                
                When size is specified as a Hp value, the W will be calculated using the current 
                aspect ratio of the video.
                When FRAMERATE is not given, the current framerate will be used.
                When QUALITY is not given, a medium vaue of 0.11 will be used.

                Examples: 
                -f 1920x1080::     	Convert into 1920x1080 maintaining frame rate and using a medium quality factor
                -f 1080p:30:		Convert into 1080p (W will be 1920 if aspect ratio is 16/9) and 30fps
                -f 360p::0.07		Convert into 360p, using a low quality factor.
```
    