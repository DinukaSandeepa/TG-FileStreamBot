package routes

import (
	"EverythingSuckz/fsb/config"
	"EverythingSuckz/fsb/internal/bot"
	"EverythingSuckz/fsb/internal/subtitles"
	"EverythingSuckz/fsb/internal/types"
	"EverythingSuckz/fsb/internal/utils"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gotd/td/tg"
	"go.uber.org/zap"
)

var subtitleLog *zap.Logger

func (e *allRoutes) LoadSubtitle(r *Route) {
	subtitleLog = e.log.Named("Subtitle")
	defer subtitleLog.Info("Loaded subtitle routes")
	r.Engine.GET("/subtitle/db/:id", getDBSubtitleRoute)
	r.Engine.GET("/subtitle/db/:id/links", getDBSubtitleLinksRoute)
}

func getDBSubtitleLinksRoute(ctx *gin.Context) {
	if !subtitles.Enabled() {
		http.Error(ctx.Writer, "subtitle repository is disabled", http.StatusServiceUnavailable)
		return
	}
	id := ctx.Param("id")
	subtitle, err := subtitles.FindByID(ctx, id)
	if err != nil {
		if err == subtitles.ErrNotFound {
			http.Error(ctx.Writer, "subtitle not found", http.StatusNotFound)
			return
		}
		http.Error(ctx.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	baseURL := buildPublicBaseURL(ctx)
	originalURL := fmt.Sprintf("%s/subtitle/db/%s", baseURL, id)
	response := gin.H{
		"id":            id,
		"url":           originalURL,
		"route":         fmt.Sprintf("/subtitle/db/%s", id),
		"language":      subtitle.Language,
		"fileName":      subtitle.FileName,
		"sourceChannel": subtitleSourceChannelID(subtitle),
	}
	if utils.IsSRTFile(subtitle.FileName, "") {
		response["vttUrl"] = originalURL + "?format=vtt"
	}
	ctx.JSON(http.StatusOK, response)
}

func getDBSubtitleRoute(ctx *gin.Context) {
	if !subtitles.Enabled() {
		http.Error(ctx.Writer, "subtitle repository is disabled", http.StatusServiceUnavailable)
		return
	}

	id := ctx.Param("id")
	subtitle, err := subtitles.FindByID(ctx, id)
	if err != nil {
		if err == subtitles.ErrNotFound {
			http.Error(ctx.Writer, "subtitle not found", http.StatusNotFound)
			return
		}
		http.Error(ctx.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	sourceChannel := subtitleSourceChannelID(subtitle)
	if sourceChannel == 0 {
		http.Error(ctx.Writer, "subtitle source channel is not configured", http.StatusInternalServerError)
		return
	}

	worker := bot.GetNextWorker()
	file, err := utils.FileFromChannelMessage(ctx, worker.Client, sourceChannel, subtitle.MessageID)
	if err != nil {
		http.Error(ctx.Writer, err.Error(), http.StatusBadRequest)
		return
	}
	if file.FileName == "" {
		file.FileName = subtitle.FileName
	}
	if file.FileName == "" {
		file.FileName = "subtitle"
	}

	content, err := fetchTelegramFileBytes(ctx, worker, file)
	if err != nil {
		http.Error(ctx.Writer, err.Error(), http.StatusInternalServerError)
		return
	}

	asVTT := shouldServeVTT(ctx)
	fileName := file.FileName
	mimeType := utils.SubtitleMimeType(file.FileName, file.MimeType)
	if asVTT {
		switch {
		case utils.IsVTTFile(fileName, mimeType):
			mimeType = "text/vtt"
		case utils.IsSRTFile(fileName, mimeType):
			content = utils.SRTToVTT(content)
			fileName = utils.SubtitleFilenameVTT(fileName)
			mimeType = "text/vtt"
		default:
			http.Error(ctx.Writer, "subtitle format is not convertible to vtt", http.StatusBadRequest)
			return
		}
	}

	disposition := "inline"
	if parseDownloadFlag(ctx.Query("d")) {
		disposition = "attachment"
	}

	ctx.Header("Cache-Control", "public, max-age=31536000, immutable")
	ctx.Header("Content-Type", mimeType)
	ctx.Header("Content-Length", fmt.Sprintf("%d", len(content)))
	ctx.Header("Content-Disposition", fmt.Sprintf("%s; filename=\"%s\"", disposition, fileName))
	if ctx.Request.Method != http.MethodHead {
		ctx.Data(http.StatusOK, mimeType, content)
		return
	}
	ctx.Status(http.StatusOK)
}

func shouldServeVTT(ctx *gin.Context) bool {
	if parseDownloadFlag(ctx.Query("vtt")) {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(ctx.Query("format")), "vtt")
}

func subtitleSourceChannelID(subtitle *subtitles.SubtitleRef) int64 {
	if subtitle.SourceChannel != 0 {
		return subtitle.SourceChannel
	}
	return config.ValueOf.SubtitleChannelID
}

func buildPublicBaseURL(ctx *gin.Context) string {
	configuredHost := strings.TrimRight(config.ValueOf.Host, "/")
	if configuredHost != "" {
		return configuredHost
	}
	proto := strings.TrimSpace(strings.Split(ctx.GetHeader("X-Forwarded-Proto"), ",")[0])
	if proto == "" {
		if ctx.Request.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}
	return fmt.Sprintf("%s://%s", proto, ctx.Request.Host)
}

func fetchTelegramFileBytes(ctx *gin.Context, worker *bot.Worker, file *types.File) ([]byte, error) {
	const chunkSize = 512 * 1024
	capacity := 0
	if file.FileSize > 0 && file.FileSize < 32*1024*1024 {
		capacity = int(file.FileSize)
	}
	data := make([]byte, 0, capacity)
	offset := int64(0)

	for {
		res, err := worker.Client.API().UploadGetFile(ctx, &tg.UploadGetFileRequest{
			Location: file.Location,
			Offset:   offset,
			Limit:    chunkSize,
		})
		if err != nil {
			return nil, err
		}
		result, ok := res.(*tg.UploadFile)
		if !ok {
			return nil, fmt.Errorf("unexpected response")
		}
		chunk := result.GetBytes()
		if len(chunk) == 0 {
			break
		}
		data = append(data, chunk...)
		offset += int64(len(chunk))
		if len(chunk) < chunkSize {
			break
		}
		if file.FileSize > 0 && offset >= file.FileSize {
			break
		}
	}

	return data, nil
}
