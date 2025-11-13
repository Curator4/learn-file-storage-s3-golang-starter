package main

import "os/exec"

// 2. Create a new function called processVideoForFastStart(filePath string) (string, error) that takes a file path as input and creates and returns a new path to a file with "fast start" encoding. It should:
func processVideoForFastStart(filePath string) (string, error) {
	// 2.1 Create a new string for the output file path. I just appended .processing to the input file (which should be the path to the temp file on disk)
	output_filepath := filePath + ".processing"

	// 2.2 Create a new exec.Cmd using exec.Command
	ffmpegCommand := exec.Command("ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", output_filepath)

	// 2.3 Run the command
	if err := ffmpegCommand.Run(); err != nil {
		return "", err
	}

	// 2.4 Return the output file path
	return output_filepath, nil
}
