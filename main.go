package main

import (
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// ffmpeg -i $input -filter:v:0 scale=w=640:h=360 -c:a aac -strict -2 -ar 48000 -c:v:0 libx264 -crf 20 -profile:v:0 main -pix_fmt yuv420p -f hls -hls_time 10 -hls_playlist_type vod -b:v:0 800k -maxrate:v:0 856k -bufsize:v:0 1200k -b:a 96k -hls_segment_filename "v360_%03d.ts" v360.m3u8

// ffmpeg -i $input -filter:v:1 scale=w=1280:h=720 -c:a aac -strict -2 -ar 48000 -c:v:1 libx264 -crf 20 -profile:v:1 main -pix_fmt yuv420p -f hls -hls_time 10 -hls_playlist_type vod -b:v:1 2800k -maxrate:v:1 2996k -bufsize:v:1 4200k -b:a 128k -hls_segment_filename "v720_%03d.ts" v720.m3u8

// ffmpeg -i $input -filter:v:2 scale=w=1920:h=1080 -c:a aac -strict -2 -ar 48000 -c:v:2 libx264 -crf 20 -profile:v:2 main -pix_fmt yuv420p -f hls -hls_time 10 -hls_playlist_type vod -b:v:2 5000k -maxrate:v:2 5350k -bufsize:v:2 7500k -b:a 192k -hls_segment_filename "v1080_%03d.ts" v1080.m3u8

type BitRates struct {
	VideoBitRate        int
	MaximumVideoBitRate int
	BufferSize          int
	AudioBitRate        int
}

type QualityFactor float64

const (
	QualityLow     QualityFactor = 0.07
	QualityLowMed  QualityFactor = 0.09
	QualityMed     QualityFactor = 0.11
	QualityMedHigh QualityFactor = 0.13
	QualityHigh    QualityFactor = 0.15
)

type ParsedFormat struct {
	Width         int
	Height        int
	FrameRate     float64
	QualityFactor float64
}

func (f *ParsedFormat) String() string {
	b := strings.Builder{}

	if f.Width == 0 {
		b.WriteString(fmt.Sprintf("%dp:", f.Height))
	} else {
		b.WriteString(fmt.Sprintf("%dx%d:", f.Width, f.Height))
	}

	if f.FrameRate != 0 {
		b.WriteString(fmt.Sprintf("%f:", f.FrameRate))
	} else {
		b.WriteString(":")
	}

	if f.QualityFactor != 0 {
		b.WriteString(fmt.Sprintf("%f:", f.QualityFactor))
	} else {
		b.WriteString(":")
	}

	return b.String()
}

func NewBitRates(f ParsedFormat) *BitRates {

	r := &BitRates{}

	r.VideoBitRate = int(float64(f.Width*f.Height) * f.FrameRate * float64(f.QualityFactor))
	r.MaximumVideoBitRate = int(.07 * float64(r.VideoBitRate))
	r.BufferSize = r.MaximumVideoBitRate * 2

	area := f.Width * f.Height

	if area <= 640*360 {
		r.AudioBitRate = 96000
	} else if area <= 1280*720 {
		r.AudioBitRate = 128000
	} else {
		r.AudioBitRate = 192000
	}

	return r
}

//Video Bitrate (kbps)=Width×Height×Frame Rate×Quality Factor
//maxrate = 1.07 * video bitrate
//bufsize = 2 * maxrate
// Audio
//96 kbps for lower quality (e.g., 360p video)
// 128 kbps for medium quality (e.g., 720p video)
// 192 kbps for higher quality (e.g., 1080p video)

//ffprobe -v error -select_streams v:0 -show_entries stream=width,height -of csv=s=x:p=0 input.mp4

// ffprobe -v error -select_streams v:0 -show_entries stream=width,height,r_frame_rate -of csv=s=,:p=0 ../../Downloads/drone.mp4
type ProbeResult struct {
	Width     int
	Height    int
	FrameRate float64
}

func Probe(inputFile string) (ProbeResult, error) {
	cmd := exec.Command(
		"ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height,r_frame_rate",
		"-of", "csv=s=,:p=0",
		inputFile,
	)
	out, err := cmd.StdoutPipe()
	if err != nil {
		return ProbeResult{}, fmt.Errorf("unable to pipe stdout: %w", err)
	}

	err = cmd.Start()
	if err != nil {
		return ProbeResult{}, fmt.Errorf("couldn't start command: %w", err)
	}

	pData, err := io.ReadAll(out)
	if err != nil {
		return ProbeResult{}, fmt.Errorf("couldn't read program output: %w", err)
	}

	err = cmd.Wait()
	if err != nil {
		log.Println("failed on command: ", cmd.String())
		return ProbeResult{}, fmt.Errorf("couldn't wait program: %w", err)
	}

	// something like streams.stream.0.r_frame_rate="24000/1001"
	res := strings.TrimSpace(string(pData))

	parts := strings.Split(res, ",")
	if len(parts) != 3 {
		return ProbeResult{}, fmt.Errorf("couldn't parse output: expected 3 parts but found %d", len(parts))
	}

	w, h, ratio := parts[0], parts[1], parts[2]

	dividend, divisor, ok := strings.Cut(ratio, "/")
	if !ok {
		return ProbeResult{}, fmt.Errorf("unexpected program output: couldn't find '/'")
	}

	pDividend, err1 := strconv.Atoi(dividend)
	pDivisor, err2 := strconv.Atoi(divisor)
	width, err3 := strconv.Atoi(w)
	height, err4 := strconv.Atoi(h)
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		return ProbeResult{}, fmt.Errorf("unable to parse probe result")
	}

	if pDivisor == 0 {
		return ProbeResult{}, fmt.Errorf("unable to calculate frame-rate, divisor causes division by zero error")
	}

	p := ProbeResult{}
	p.FrameRate = float64(pDividend) / float64(pDivisor)
	p.Width = width
	p.Height = height
	return p, nil

}

func convert(inputFile string, w int, h int, fps float64, r BitRates) (string, error) {

	outputFileName := fmt.Sprintf("v%dx%d.m3u8", w, h)

	cmd := exec.Command("ffmpeg",
		"-i", inputFile,
		"-filter:v", fmt.Sprintf("scale=w=%d:h=%d,fps=%f", w, h, fps),
		"-c:a", "aac",
		"-strict", "-2",
		"-ar", "48000",
		"-c:v", "libx264",
		"-crf", "20",
		"-profile:v", "main",
		"-pix_fmt", "yuv420p",
		"-f", "hls",
		"-hls_time", "10",
		"-hls_playlist_type", "vod",
		"-b:v", fmt.Sprint(r.VideoBitRate),
		"-maxrate", fmt.Sprint(r.MaximumVideoBitRate),
		"-bufsize:v", fmt.Sprint(r.BufferSize),
		"-b:a", fmt.Sprint(r.AudioBitRate),
		"-hls_segment_filename",
		fmt.Sprintf("v%dx%d_%%03d.ts", w, h),
		outputFileName,
	)

	err := cmd.Run()
	if err != nil {
		log.Println("failed on command:", cmd.String())
	}

	return outputFileName, err
}

type Stream struct {
	Width     int
	Height    int
	Bandwidth int
	FileName  string
}

func makeMasterTrack(masterName string, streams []Stream) error {

	f, err := os.Create(masterName)
	if err != nil {
		return fmt.Errorf("couldn't open file")
	}
	defer f.Close()

	b := strings.Builder{}
	b.WriteString("#EXTM3U\n")

	for _, s := range streams {
		b.WriteString(fmt.Sprintf("#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d\n", s.Bandwidth, s.Width, s.Height))
		b.WriteString(s.FileName)
		b.WriteString("\n")
	}

	_, err = f.WriteString(b.String())
	if err != nil {
		return fmt.Errorf("unable to write file: %w", err)
	}

	return nil

}

func usage() {
	fmt.Fprint(os.Stderr, `tohls OPTIONS FILES
	
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

`)

}

func parseArgs(args []string) (rawFormats, inputFiles []string, err error) {

	rawFormats = []string{}
	inputFiles = []string{}

	// can be 'any', 'format', 'file'
	expects := "any"
	for i := 0; i < len(args); i++ {
		switch expects {
		case "any":
			if args[i] == "-f" {
				expects = "format"
			} else if strings.HasPrefix(args[i], "-f=") {
				p := strings.TrimPrefix(args[i], "-f=")
				rawFormats = append(rawFormats, p)
			} else {
				expects = "file"
				inputFiles = append(inputFiles, args[i])
			}
		case "format":
			rawFormats = append(rawFormats, args[i])
			expects = "any"

		case "file":
			inputFiles = append(inputFiles, args[i])
		default:
			panic("invalid expect")
		}
	}

	if len(rawFormats) == 0 {
		return nil, nil, fmt.Errorf("no formats given")
	}

	if len(inputFiles) == 0 {
		return nil, nil, fmt.Errorf("no input files given")
	}

	return
}

func parseFormat(v string) (ParsedFormat, error) {
	p := ParsedFormat{}

	parts := strings.Split(v, ":")
	if len(parts) != 3 {
		return ParsedFormat{}, fmt.Errorf("malformed format")
	}

	size := parts[0]
	frameRate := parts[1]
	qualityFactor := parts[2]

	var err error
	// parse size
	{
		if strings.HasSuffix(size, "p") {
			size = strings.TrimSuffix(size, "p")
			p.Height, err = strconv.Atoi(size)
			if err != nil {
				return ParsedFormat{}, fmt.Errorf("malformed format")
			}
		} else {
			w, h, ok := strings.Cut(size, "x")
			if !ok {
				return ParsedFormat{}, fmt.Errorf("malformed format")
			}

			p.Width, err = strconv.Atoi(w)
			if err != nil {
				return ParsedFormat{}, fmt.Errorf("malformed format")
			}
			p.Height, err = strconv.Atoi(h)
			if err != nil {
				return ParsedFormat{}, fmt.Errorf("malformed format")
			}
		}
	}
	// parse frameRate
	{
		if frameRate != "" {
			p.FrameRate, err = strconv.ParseFloat(frameRate, 64)
			if err != nil {
				return ParsedFormat{}, fmt.Errorf("malformed format")
			}
		}
	}
	// parse qualityFactor
	{
		if qualityFactor != "" {
			p.QualityFactor, err = strconv.ParseFloat(qualityFactor, 64)
			if err != nil {
				return ParsedFormat{}, fmt.Errorf("malformed format")
			}
		}
	}

	return p, nil
}

func processFile(formats []ParsedFormat, inputFile string) error {

	fmt.Printf("Processing file '%s'\n", inputFile)

	probe, err := Probe(inputFile)
	if err != nil {
		return fmt.Errorf("couldn't probe file: %w", err)
	}

	adjustedFormats := make([]ParsedFormat, len(formats))
	for i := 0; i < len(formats); i++ {

		adjustedFormats[i].Height = formats[i].Height
		if formats[i].Width == 0 {
			// compute width based on aspect ratio
			adjustedFormats[i].Width = int(math.Round(float64(probe.Width) / float64(probe.Height) * float64(formats[i].Height)))
			if adjustedFormats[i].Width%2 != 0 {
				adjustedFormats[i].Width += 1
			}
		}

		if formats[i].FrameRate == 0 {
			adjustedFormats[i].FrameRate = probe.FrameRate
		}

		if formats[i].QualityFactor == 0 {
			formats[i].QualityFactor = float64(QualityMed)
		}

	}

	streams := []Stream{}

	for i, format := range adjustedFormats {

		rates := NewBitRates(format)

		fmt.Printf("\tformat '%s'\n", formats[i].String())
		fileName, err := convert(inputFile, format.Width, format.Height, format.FrameRate, *rates)
		if err != nil {
			return fmt.Errorf("unable to convert: %w", err)
		}

		streams = append(streams, Stream{
			Width:     format.Width,
			Height:    format.Height,
			Bandwidth: rates.VideoBitRate,
			FileName:  fileName,
		})
	}

	base := filepath.Base(inputFile)
	masterTrack := fmt.Sprintf("%s.master.m3u8", base)

	fmt.Printf("\tjoining into '%s'\n", masterTrack)
	err = makeMasterTrack(masterTrack, streams)
	if err != nil {
		return fmt.Errorf("unable to join: %w", err)
	}

	fmt.Printf("\tdone")
	return nil
}

func main() {

	rawFormats, inputFiles, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err.Error())
		usage()
		os.Exit(1)
	}

	formats := make([]ParsedFormat, len(rawFormats))

	for i := 0; i < len(formats); i++ {
		formats[i], err = parseFormat(rawFormats[i])
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err.Error())
			os.Exit(1)
		}
	}

	for _, v := range inputFiles {
		err = processFile(formats, v)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error while processing file '%s': %s", v, err.Error())
			os.Exit(1)
		}
	}
}
