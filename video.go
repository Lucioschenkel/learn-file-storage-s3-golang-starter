package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
)

type stream struct {
	Height int `json:"height"`
	Width  int `json:"width"`
}

type VideoMetadata struct {
	Streams []stream `json:"streams"`
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}

	return a
}

func getVideoAspectRatio(filePath string) (string, error) {
	var stdOut bytes.Buffer
	var result VideoMetadata

	fmt.Println("Determining aspect ratio for", filePath)
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)
	cmd.Stdout = &stdOut
	err := cmd.Run()

	if err != nil {
		return "", err
	}

	err = json.Unmarshal(stdOut.Bytes(), &result)
	if err != nil {
		return "", err
	}

	stream0 := result.Streams[0]
	g := gcd(stream0.Width, stream0.Height)
	x, y := stream0.Width/g, stream0.Height/g
	fmt.Printf("Width: %d, Height: %d, g: %d\n", stream0.Width, stream0.Height, g)

	aspectRatio := fmt.Sprintf("%d:%d", stream0.Width/g, stream0.Height/g)
	if y > x {
		return "9:16", nil
	}

	return aspectRatio, nil
}

func processVideoForFastStart(filePath string) (string, error) {
	outputFilePath := filePath + ".processing"
	cmd := exec.Command("ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outputFilePath)
	err := cmd.Run()
	if err != nil {
		return "", err
	}

	return outputFilePath, nil
}
