package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading video file for video", videoID, "by user", userID)

	const maxMemory = 1 << 30

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Couldn't find video with id "+videoIDString, err)
		return
	}
	if video.UserID.String() != userID.String() {
		respondWithError(w, http.StatusUnauthorized, "Video does not belong to authenticated user", err)
		return
	}

	r.ParseMultipartForm(maxMemory)

	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to parse video file", err)
		return
	}
	defer file.Close()

	mediaType := header.Header.Get("Content-Type")
	parsedMediaType, _, err := mime.ParseMediaType(mediaType)
	if parsedMediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Invalid media type", err)
		return
	}

	tempFile, err := os.CreateTemp("/tmp", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to upload file", err)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	_, err = io.Copy(tempFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to copy file", err)
		return
	}

	processedFilePath, err := processVideoForFastStart(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to process video for fast start", err)
		return
	}

	aspectRatio, err := getVideoAspectRatio(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to determine aspect ratio", err)
		return
	}

	fmt.Println("File aspect ratio:", aspectRatio)

	var prefix string
	switch aspectRatio {
	case "16:9":
		prefix = "landscape"
	case "9:16":
		prefix = "portrait"
	default:
		prefix = "other"
	}

	extension := strings.Split(mediaType, "/")[1]
	var fileID [32]byte
	_, err = rand.Read(fileID[:])
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create video file ID", err)
		return
	}

	processedFile, err := os.Open(processedFilePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't read processed file", err)
		return
	}
	defer processedFile.Close()

	assetName := base64.RawURLEncoding.EncodeToString(fileID[:]) + "." + extension
	objectKey := prefix + "/" + assetName
	_, err = cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &objectKey,
		ContentType: &parsedMediaType,
		Body:        processedFile,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to store the video file in S3", err)
		return
	}

	videoURL := cfg.s3CfDistribution + "/" + objectKey
	video.VideoURL = &videoURL

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to update video record", err)
		return
	}

	err = os.Remove(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to delete temp file", err)
		return
	}

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate pre-signed URL", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
