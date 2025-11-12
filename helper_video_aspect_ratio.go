package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
)

func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)

	var buf bytes.Buffer
	cmd.Stdout = &buf

	if err := cmd.Run(); err != nil {
		return "", err
	}

	var output struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"streams"`
	}

	err := json.Unmarshal(buf.Bytes(), &output)
	if err != nil {
		return "", err
	}

	if len(output.Streams) == 0 {
		return "", fmt.Errorf("no streams")
	}

	width := output.Streams[0].Width
	height := output.Streams[0].Height

	// aspect ratio
	ratio := float64(width) / float64(height)

	if ratio > 1.7 && ratio < 1.85 {
		return "landscape", nil
	} else if ratio > 0.5 && ratio < 0.6 {
		return "portrait", nil
	}

	return "other", nil
}
