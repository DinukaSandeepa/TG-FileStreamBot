package routes

import (
	"EverythingSuckz/fsb/config"
	"EverythingSuckz/fsb/internal/auth"
	"EverythingSuckz/fsb/internal/bot"
	"EverythingSuckz/fsb/internal/movies"
	"EverythingSuckz/fsb/internal/stream"
	"EverythingSuckz/fsb/internal/types"
	"EverythingSuckz/fsb/internal/utils"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gotd/td/tg"
	range_parser "github.com/quantumsheep/range-parser"
	"go.uber.org/zap"

	"github.com/gin-gonic/gin"
)

var log *zap.Logger

func (e *allRoutes) LoadHome(r *Route) {
	log = e.log.Named("Stream")
	defer log.Info("Loaded stream route")
	r.Engine.GET("/stream/:messageID", getLegacyStreamRoute)
	r.Engine.GET("/stream/db/:id", getDBStreamRoute)
	r.Engine.GET("/sign/db/:id", getSignedDBLinkRoute)
}

func getLegacyStreamRoute(ctx *gin.Context) {
	w := ctx.Writer

	messageIDParm := ctx.Param("messageID")
	messageID, err := strconv.Atoi(messageIDParm)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	authHash := ctx.Query("hash")
	if authHash == "" {
		http.Error(w, "missing hash param", http.StatusBadRequest)
		return
	}

	worker := bot.GetNextWorker()

	file, err := utils.TimeFuncWithResult(log, "FileFromMessage", func() (*types.File, error) {
		return utils.FileFromMessage(ctx, worker.Client, messageID)
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	expectedHash := utils.PackFile(
		file.FileName,
		file.FileSize,
		file.MimeType,
		file.ID,
	)
	if !utils.CheckHash(authHash, expectedHash) {
		http.Error(w, "invalid hash", http.StatusBadRequest)
		return
	}

	serveResolvedFile(ctx, worker, file, ctx.Query("d") == "true")
}

func getSignedDBLinkRoute(ctx *gin.Context) {
	if !movies.Enabled() {
		http.Error(ctx.Writer, "mongo repository is disabled", http.StatusServiceUnavailable)
		return
	}
	if config.ValueOf.LinkSignAPIKey == "" || config.ValueOf.StreamSigningSecret == "" {
		http.Error(ctx.Writer, "sign endpoint is disabled", http.StatusServiceUnavailable)
		return
	}
	if !isSignRequestAuthorized(ctx) {
		http.Error(ctx.Writer, "unauthorized", http.StatusUnauthorized)
		return
	}

	id := ctx.Param("id")
	if _, err := movies.FindByID(ctx, id); err != nil {
		if errors.Is(err, movies.ErrNotFound) {
			http.Error(ctx.Writer, "movie not found", http.StatusNotFound)
			return
		}
		http.Error(ctx.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	download := parseDownloadFlag(ctx.Query("d"))
	ttl := time.Duration(config.ValueOf.StreamTokenTTLSec) * time.Second
	now := time.Now()
	token, err := auth.SignStreamToken(id, download, ttl, config.ValueOf.StreamSigningSecret, now)
	if err != nil {
		http.Error(ctx.Writer, err.Error(), http.StatusInternalServerError)
		return
	}
	streamURL := auth.BuildStreamDBURL(config.ValueOf.Host, id, token)
	ctx.JSON(http.StatusOK, gin.H{
		"id":         id,
		"url":        streamURL,
		"download":   download,
		"expiresAt":  now.Add(ttl).Unix(),
		"expiresIn":  int(ttl.Seconds()),
		"token":      token,
		"route":      fmt.Sprintf("/stream/db/%s", id),
		"tokenParam": "token",
	})
}

func getDBStreamRoute(ctx *gin.Context) {
	if !movies.Enabled() {
		http.Error(ctx.Writer, "mongo repository is disabled", http.StatusServiceUnavailable)
		return
	}
	if config.ValueOf.StreamSigningSecret == "" {
		http.Error(ctx.Writer, "stream signing is disabled", http.StatusServiceUnavailable)
		return
	}

	id := ctx.Param("id")
	token := ctx.Query("token")
	if token == "" {
		http.Error(ctx.Writer, "missing token param", http.StatusBadRequest)
		return
	}
	claims, err := auth.VerifyStreamToken(token, id, config.ValueOf.StreamSigningSecret, time.Now())
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrTokenExpired):
			http.Error(ctx.Writer, "token expired", http.StatusUnauthorized)
		case errors.Is(err, auth.ErrIDMismatch), errors.Is(err, auth.ErrInvalidSignature):
			http.Error(ctx.Writer, "invalid token", http.StatusForbidden)
		default:
			http.Error(ctx.Writer, "invalid token", http.StatusBadRequest)
		}
		return
	}

	movie, err := movies.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, movies.ErrNotFound) {
			http.Error(ctx.Writer, "movie not found", http.StatusNotFound)
			return
		}
		http.Error(ctx.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	worker := bot.GetNextWorker()
	file, err := utils.TimeFuncWithResult(log, "FileFromChannelMessage", func() (*types.File, error) {
		return utils.FileFromChannelMessage(ctx, worker.Client, movie.SourceChannel, movie.MessageID)
	})
	if err != nil {
		http.Error(ctx.Writer, err.Error(), http.StatusBadRequest)
		return
	}
	if file.FileName == "" && movie.FileName != "" {
		file.FileName = movie.FileName
	}

	serveResolvedFile(ctx, worker, file, claims.Download)
}

func serveResolvedFile(ctx *gin.Context, worker *bot.Worker, file *types.File, download bool) {
	w := ctx.Writer
	r := ctx.Request

	// for photo messages
	if file.FileSize == 0 {
		res, err := worker.Client.API().UploadGetFile(ctx, &tg.UploadGetFileRequest{
			Location: file.Location,
			Offset:   0,
			Limit:    1024 * 1024,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		result, ok := res.(*tg.UploadFile)
		if !ok {
			http.Error(w, "unexpected response", http.StatusInternalServerError)
			return
		}
		fileBytes := result.GetBytes()
		ctx.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", file.FileName))
		if r.Method != "HEAD" {
			ctx.Data(http.StatusOK, file.MimeType, fileBytes)
		}
		return
	}

	ctx.Header("Accept-Ranges", "bytes")
	var start, end int64
	rangeHeader := r.Header.Get("Range")

	if rangeHeader == "" {
		start = 0
		end = file.FileSize - 1
		if !download {
			if cappedEnd, capped := capPlaybackRangeEnd(start, end, file.FileSize); capped {
				end = cappedEnd
				ctx.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, file.FileSize))
				log.Info("Content-Range", zap.Int64("start", start), zap.Int64("end", end), zap.Int64("fileSize", file.FileSize), zap.Bool("capped", true), zap.String("reason", "no-range-request"))
				w.WriteHeader(http.StatusPartialContent)
			} else {
				w.WriteHeader(http.StatusOK)
			}
		} else {
			w.WriteHeader(http.StatusOK)
		}
	} else {
		ranges, err := range_parser.Parse(file.FileSize, r.Header.Get("Range"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		start = ranges[0].Start
		end = ranges[0].End
		if !download {
			if cappedEnd, capped := capPlaybackRangeEnd(start, end, file.FileSize); capped {
				log.Info("capped playback range", zap.String("requestedRange", rangeHeader), zap.Int64("start", start), zap.Int64("requestedEnd", end), zap.Int64("cappedEnd", cappedEnd), zap.Int("chunkMB", config.ValueOf.StreamOpenEndedChunkMB))
				end = cappedEnd
			}
		}
		ctx.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, file.FileSize))
		log.Info("Content-Range", zap.Int64("start", start), zap.Int64("end", end), zap.Int64("fileSize", file.FileSize), zap.Bool("capped", end < ranges[0].End))
		w.WriteHeader(http.StatusPartialContent)
	}

	contentLength := end - start + 1
	mimeType := file.MimeType

	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	ctx.Header("Content-Type", mimeType)
	ctx.Header("Content-Length", strconv.FormatInt(contentLength, 10))

	disposition := "inline"

	if download {
		disposition = "attachment"
	}

	ctx.Header("Content-Disposition", fmt.Sprintf("%s; filename=\"%s\"", disposition, file.FileName))

	if r.Method != "HEAD" {
		pipe, err := stream.NewStreamPipe(ctx, worker.Client, file.Location, start, end, log)
		if err != nil {
			log.Error("Failed to create stream pipe", zap.Error(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer pipe.Close()
		if _, err := io.CopyN(w, pipe, contentLength); err != nil {
			if !utils.IsClientDisconnectError(err) {
				log.Error("Error while copying stream", zap.Error(err))
			}
		}
	}
}

func parseDownloadFlag(v string) bool {
	v = strings.TrimSpace(strings.ToLower(v))
	return v == "1" || v == "true" || v == "yes"
}

func capPlaybackRangeEnd(start, end, fileSize int64) (int64, bool) {
	if config.ValueOf.StreamOpenEndedChunkMB <= 0 {
		return end, false
	}

	capBytes := int64(config.ValueOf.StreamOpenEndedChunkMB) * 1024 * 1024
	if capBytes <= 0 {
		return end, false
	}

	cappedEnd := start + capBytes - 1
	if cappedEnd > end {
		cappedEnd = end
	}
	if cappedEnd > fileSize-1 {
		cappedEnd = fileSize - 1
	}
	if cappedEnd < start {
		return end, false
	}

	if cappedEnd < end {
		return cappedEnd, true
	}

	return end, false
}

func isSignRequestAuthorized(ctx *gin.Context) bool {
	expected := config.ValueOf.LinkSignAPIKey
	if expected == "" {
		return false
	}
	apiKey := ctx.GetHeader("X-API-Key")
	if secureCompare(apiKey, expected) {
		return true
	}
	authorization := strings.TrimSpace(ctx.GetHeader("Authorization"))
	if strings.HasPrefix(strings.ToLower(authorization), "bearer ") {
		bearerToken := strings.TrimSpace(authorization[len("Bearer "):])
		if secureCompare(bearerToken, expected) {
			return true
		}
	}
	return false
}

func secureCompare(a, b string) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
