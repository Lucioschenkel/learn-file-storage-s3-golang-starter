package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
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

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	const maxMemory = 10 << 20
	r.ParseMultipartForm(maxMemory)

	file, header, err := r.FormFile("thumbnail")
	mediaType := header.Header.Get("Content-Type")
	parsedMediaType, _, err := mime.ParseMediaType(mediaType)
	if err != nil || (parsedMediaType != "image/jpeg" && parsedMediaType != "image/png") {
		respondWithError(w, http.StatusBadRequest, "Invalid media type", err)
	}

	extension := strings.Split(mediaType, "/")[1]
	var fileID [32]byte
	_, err = rand.Read(fileID[:])
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create thumbnail file ID", err)
		return
	}

	assetName := base64.RawURLEncoding.EncodeToString(fileID[:]) + "." + extension
	filePath := filepath.Join(cfg.assetsRoot, assetName)
	destFile, err := os.Create(filePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create thumbnail file", err)
		return
	}

	_, err = io.Copy(destFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't store thumbnail", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Couldn't find video with id "+videoIDString, err)
		return
	}
	if video.UserID.String() != userID.String() {
		respondWithError(w, http.StatusUnauthorized, "Video does not belong to authenticated user", err)
		return
	}

	thumbnailURL := "http://localhost:" + cfg.port + "/assets/" + assetName
	video.ThumbnailURL = &thumbnailURL

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to update video record", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
