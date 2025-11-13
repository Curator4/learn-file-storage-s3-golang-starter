package main

import (
	"strings"
	"time"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
)

const expireTime = 10 * time.Second

// ch6l6 5. Create a new (cfg *apiConfig) dbVideoToSignedVideo(video database.Video) (database.Video, error) method:
func (cfg *apiConfig) dbVideoToSignedVideo(video database.Video) (database.Video, error) {
	if video.VideoURL == nil || *video.VideoURL == "" {
		return video, nil
	}

	bucket, key, found := strings.Cut(*video.VideoURL, ",")

	if !found || key == "" {
		return video, nil
	}

	presignedURL, err := generatePresignedURL(cfg.s3Client, bucket, key, expireTime)
	if err != nil {
		return database.Video{}, err
	}
	video.VideoURL = &presignedURL

	return video, nil
}
