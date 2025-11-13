package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	// 1. Set an upload limit of 1 GB (1 << 30 bytes) using http.MaxBytesReader.
	const uploadLimit = 1 << 30

	// 2. Extract the videoID from the URL path parameters and parse it as a UUID
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid id", err)
		return
	}

	// 3. Authenticate the user to get a userID
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "couldn't find jwt", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "couldn't validate jwt", err)
		return
	}

	// 4. Get the video metadata from the database, if the user is not the video owner, return a http.StatusUnauthorized response
	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't find video", err)
		return
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "not authorized to update this video", nil)
		return
	}

	// 5. Parse the uploaded video file from the form data
	r.ParseMultipartForm(uploadLimit)
	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "unable to parse formfile - video", err)
		return
	}
	defer file.Close()

	// 6. Validate the uploaded file to ensure it's an MP4 video
	contentType := header.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid content type - video", err)
		return
	}
	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "invalid mediatype, needs video/mp4", err)
		return
	}

	// 7. Save the uploaded file to a temporary file on disk.
	const tempFilename = "tubely-upload.mp4"
	tempFile, err := os.CreateTemp("", tempFilename)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to create temp file", err)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	_, err = io.Copy(tempFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed file copy", err)
		return
	}

	// ch4 - Update the handlerUploadVideo to get the aspect ratio of the video file from the temporary file once it's saved to disk. Depending on the aspect ratio, add a "landscape", "portrait", or "other" prefix to the key before uploading it to S3.
	aspectRatio, err := getVideoAspectRatio(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to get aspect ratio from video", err)
		return
	}

	// ch5 - Update handlerUploadVideo to create a processed version of the video. Upload the processed video to S3, and discard the original.
	processedTempFileName, err := processVideoForFastStart(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to process video moov", err)
		return
	}
	processedTempFile, err := os.Open(processedTempFileName)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to open processed video", err)
		return
	}
	defer os.Remove(processedTempFile.Name())
	defer processedTempFile.Close()

	// 8. Reset the tempFile's file pointer to the beginning with .Seek(0, io.SeekStart) - this will allow us to read the file again from the beginning
	processedTempFile.Seek(0, io.SeekStart)

	// 9. Put the object into S3 using PutObject.
	fileKeyBytes := make([]byte, 32)
	_, err = rand.Read(fileKeyBytes)
	fileKeyRaw := base64.RawURLEncoding.EncodeToString(fileKeyBytes)
	fileKey := fmt.Sprintf("%s/%s", aspectRatio, fileKeyRaw)

	objectParams := s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &fileKey,
		Body:        processedTempFile,
		ContentType: &mediaType,
	}
	_, err = cfg.s3Client.PutObject(context.Background(), &objectParams)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to upload video to aws", err)
		return
	}

	videoURL := fmt.Sprintf("https://%s/%s", cfg.s3CfDistribution, fileKey)
	video.VideoURL = &videoURL

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
