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

	// 8. Reset the tempFile's file pointer to the beginning with .Seek(0, io.SeekStart) - this will allow us to read the file again from the beginning
	tempFile.Seek(0, io.SeekStart)

	// 9. Put the object into S3 using PutObject.
	fileKeyBytes := make([]byte, 32)
	_, err = rand.Read(fileKeyBytes)
	fileKey := base64.RawURLEncoding.EncodeToString(fileKeyBytes)

	objectParams := s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &fileKey,
		Body:        tempFile,
		ContentType: &mediaType,
	}
	_, err = cfg.s3Client.PutObject(context.Background(), &objectParams)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to upload video to aws", err)
		return
	}

	// 10. Update the VideoURL of the video record in the database with the S3 bucket and key. S3 URLs are in the format https://<bucket-name>.s3.<region>.amazonaws.com/<key>. Make sure you use the correct region and bucket name!
	s3URL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, fileKey)
	video.VideoURL = &s3URL

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
