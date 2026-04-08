package config

import (
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var ValueOf = &config{}

type allowedUsers []int64

func (au *allowedUsers) Decode(value string) error {
	if value == "" {
		return nil
	}
	ids := strings.Split(string(value), ",")
	for _, id := range ids {
		idInt, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			return err
		}
		*au = append(*au, idInt)
	}
	return nil
}

type config struct {
	ApiID             int32        `envconfig:"API_ID" required:"true"`
	ApiHash           string       `envconfig:"API_HASH" required:"true"`
	BotToken          string       `envconfig:"BOT_TOKEN" required:"true"`
	LogChannelID      int64        `envconfig:"LOG_CHANNEL" default:"0"`
	SubtitleChannelID int64        `envconfig:"SUBTITLE_CHANNEL_ID" default:"0"`
	Dev               bool         `envconfig:"DEV" default:"false"`
	Port              int          `envconfig:"PORT" default:"8080"`
	Host              string       `envconfig:"HOST" default:""`
	HashLength        int          `envconfig:"HASH_LENGTH" default:"6"`
	UseSessionFile    bool         `envconfig:"USE_SESSION_FILE" default:"true"`
	UserSession       string       `envconfig:"USER_SESSION"`
	UsePublicIP       bool         `envconfig:"USE_PUBLIC_IP" default:"false"`
	AllowedUsers      allowedUsers `envconfig:"ALLOWED_USERS"`
	MultiTokens       []string

	// Mongo and signed-link config
	MongoURI                 string `envconfig:"MONGO_URI" default:""`
	MongoDB                  string `envconfig:"MONGO_DB" default:""`
	MongoCollection          string `envconfig:"MONGO_COLLECTION" default:"movies"`
	MongoSubtitlesCollection string `envconfig:"MONGO_SUBTITLES_COLLECTION" default:"subtitles"`
	StreamSigningSecret      string `envconfig:"STREAM_SIGNING_SECRET" default:""`
	StreamTokenTTLSec        int    `envconfig:"STREAM_TOKEN_TTL_SEC" default:"1800"`
	LinkSignAPIKey           string `envconfig:"LINK_SIGN_API_KEY" default:""`

	// stream specific config
	StreamConcurrency      int `envconfig:"STREAM_CONCURRENCY" default:"4"`
	StreamBufferCount      int `envconfig:"STREAM_BUFFER_COUNT" default:"8"`
	StreamInitialBufferMB  int `envconfig:"STREAM_INITIAL_BUFFER_MB" default:"4"`
	StreamOpenEndedChunkMB int `envconfig:"STREAM_OPEN_ENDED_CHUNK_MB" default:"0"`
	StreamTimeoutSec       int `envconfig:"STREAM_TIMEOUT_SEC" default:"30"`
	StreamMaxRetries       int `envconfig:"STREAM_MAX_RETRIES" default:"3"`
}

var botTokenRegex = regexp.MustCompile(`MULTI\_TOKEN\d+=(.*)`)

func (c *config) loadFromEnvFile(log *zap.Logger) {
	envPath := filepath.Clean("fsb.env")
	log.Sugar().Infof("Trying to load ENV vars from %s", envPath)
	err := godotenv.Load(envPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Sugar().Errorf("ENV file not found: %s", envPath)
			log.Sugar().Info("Please create fsb.env file")
			log.Sugar().Info("For more info, refer: https://github.com/EverythingSuckz/TG-FileStreamBot/tree/golang#setting-up-things")
			log.Sugar().Info("Please ignore this message if you are hosting it in a service like Heroku or other alternatives.")
		} else {
			log.Fatal("Unknown error while parsing env file.", zap.Error(err))
		}
	}
}

func SetFlagsFromConfig(cmd *cobra.Command) {
	cmd.Flags().Int32("api-id", ValueOf.ApiID, "Telegram API ID")
	cmd.Flags().String("api-hash", ValueOf.ApiHash, "Telegram API Hash")
	cmd.Flags().String("bot-token", ValueOf.BotToken, "Telegram Bot Token")
	cmd.Flags().Int64("log-channel", ValueOf.LogChannelID, "Telegram Log Channel ID")
	cmd.Flags().Int64("subtitle-channel", ValueOf.SubtitleChannelID, "Telegram Subtitle Channel ID")
	cmd.Flags().Bool("dev", ValueOf.Dev, "Enable development mode")
	cmd.Flags().IntP("port", "p", ValueOf.Port, "Server port")
	cmd.Flags().String("host", ValueOf.Host, "Server host that will be included in links")
	cmd.Flags().Int("hash-length", ValueOf.HashLength, "Hash length in links")
	cmd.Flags().Bool("use-session-file", ValueOf.UseSessionFile, "Use session files")
	cmd.Flags().String("user-session", ValueOf.UserSession, "Pyrogram user session")
	cmd.Flags().Bool("use-public-ip", ValueOf.UsePublicIP, "Use public IP instead of local IP")
	cmd.Flags().String("multi-token-txt-file", "", "Multi token txt file (Not implemented)")
	cmd.Flags().String("mongo-uri", ValueOf.MongoURI, "MongoDB connection string")
	cmd.Flags().String("mongo-db", ValueOf.MongoDB, "MongoDB database name")
	cmd.Flags().String("mongo-collection", ValueOf.MongoCollection, "MongoDB collection name for file metadata")
	cmd.Flags().String("mongo-subtitles-collection", ValueOf.MongoSubtitlesCollection, "MongoDB collection name for subtitles")
	cmd.Flags().String("stream-signing-secret", ValueOf.StreamSigningSecret, "HMAC secret used to sign DB stream tokens")
	cmd.Flags().Int("stream-token-ttl-sec", ValueOf.StreamTokenTTLSec, "Signed stream token TTL in seconds")
	cmd.Flags().String("link-sign-api-key", ValueOf.LinkSignAPIKey, "API key required for link signing endpoint")
	cmd.Flags().Int("stream-concurrency", ValueOf.StreamConcurrency, "Number of parallel block fetches")
	cmd.Flags().Int("stream-buffer-count", ValueOf.StreamBufferCount, "Number of blocks to prefetch")
	cmd.Flags().Int("stream-initial-buffer-mb", ValueOf.StreamInitialBufferMB, "How many MB to prebuffer before first bytes are sent")
	cmd.Flags().Int("stream-open-ended-chunk-mb", ValueOf.StreamOpenEndedChunkMB, "Cap open-ended Range requests to this many MB (0 disables)")
	cmd.Flags().Int("stream-timeout-sec", ValueOf.StreamTimeoutSec, "Maximum time to wait for a single block (in seconds)")
	cmd.Flags().Int("stream-max-retries", ValueOf.StreamMaxRetries, "Number of retry attempts for failed fetches")
}

func (c *config) loadConfigFromArgs(log *zap.Logger, cmd *cobra.Command) {
	apiID, _ := cmd.Flags().GetInt32("api-id")
	if apiID != 0 {
		os.Setenv("API_ID", strconv.Itoa(int(apiID)))
	}
	apiHash, _ := cmd.Flags().GetString("api-hash")
	if apiHash != "" {
		os.Setenv("API_HASH", apiHash)
	}
	botToken, _ := cmd.Flags().GetString("bot-token")
	if botToken != "" {
		os.Setenv("BOT_TOKEN", botToken)
	}
	logChannelID, _ := cmd.Flags().GetString("log-channel")
	if logChannelID != "" {
		os.Setenv("LOG_CHANNEL", logChannelID)
	}
	subtitleChannelID, _ := cmd.Flags().GetInt64("subtitle-channel")
	if subtitleChannelID != 0 {
		os.Setenv("SUBTITLE_CHANNEL_ID", strconv.FormatInt(subtitleChannelID, 10))
	}
	dev, _ := cmd.Flags().GetBool("dev")
	if dev {
		os.Setenv("DEV", strconv.FormatBool(dev))
	}
	port, _ := cmd.Flags().GetInt("port")
	if port != 0 {
		os.Setenv("PORT", strconv.Itoa(port))
	}
	host, _ := cmd.Flags().GetString("host")
	if host != "" {
		os.Setenv("HOST", host)
	}
	hashLength, _ := cmd.Flags().GetInt("hash-length")
	if hashLength != 0 {
		os.Setenv("HASH_LENGTH", strconv.Itoa(hashLength))
	}
	useSessionFile, _ := cmd.Flags().GetBool("use-session-file")
	if useSessionFile {
		os.Setenv("USE_SESSION_FILE", strconv.FormatBool(useSessionFile))
	}
	userSession, _ := cmd.Flags().GetString("user-session")
	if userSession != "" {
		os.Setenv("USER_SESSION", userSession)
	}
	usePublicIP, _ := cmd.Flags().GetBool("use-public-ip")
	if usePublicIP {
		os.Setenv("USE_PUBLIC_IP", strconv.FormatBool(usePublicIP))
	}
	multiTokens, _ := cmd.Flags().GetString("multi-token-txt-file")
	if multiTokens != "" {
		os.Setenv("MULTI_TOKEN_TXT_FILE", multiTokens)
		// TODO: Add support for importing tokens from a separate file
	}
	mongoURI, _ := cmd.Flags().GetString("mongo-uri")
	if mongoURI != "" {
		os.Setenv("MONGO_URI", mongoURI)
	}
	mongoDB, _ := cmd.Flags().GetString("mongo-db")
	if mongoDB != "" {
		os.Setenv("MONGO_DB", mongoDB)
	}
	mongoCollection, _ := cmd.Flags().GetString("mongo-collection")
	if mongoCollection != "" {
		os.Setenv("MONGO_COLLECTION", mongoCollection)
	}
	mongoSubtitlesCollection, _ := cmd.Flags().GetString("mongo-subtitles-collection")
	if mongoSubtitlesCollection != "" {
		os.Setenv("MONGO_SUBTITLES_COLLECTION", mongoSubtitlesCollection)
	}
	streamSigningSecret, _ := cmd.Flags().GetString("stream-signing-secret")
	if streamSigningSecret != "" {
		os.Setenv("STREAM_SIGNING_SECRET", streamSigningSecret)
	}
	streamTokenTTLSec, _ := cmd.Flags().GetInt("stream-token-ttl-sec")
	if streamTokenTTLSec != 0 {
		os.Setenv("STREAM_TOKEN_TTL_SEC", strconv.Itoa(streamTokenTTLSec))
	}
	linkSignAPIKey, _ := cmd.Flags().GetString("link-sign-api-key")
	if linkSignAPIKey != "" {
		os.Setenv("LINK_SIGN_API_KEY", linkSignAPIKey)
	}
	streamConcurrency, _ := cmd.Flags().GetInt("stream-concurrency")
	if streamConcurrency != 0 {
		os.Setenv("STREAM_CONCURRENCY", strconv.Itoa(streamConcurrency))
	}
	streamBufferCount, _ := cmd.Flags().GetInt("stream-buffer-count")
	if streamBufferCount != 0 {
		os.Setenv("STREAM_BUFFER_COUNT", strconv.Itoa(streamBufferCount))
	}
	streamInitialBufferMB, _ := cmd.Flags().GetInt("stream-initial-buffer-mb")
	if cmd.Flags().Changed("stream-initial-buffer-mb") {
		os.Setenv("STREAM_INITIAL_BUFFER_MB", strconv.Itoa(streamInitialBufferMB))
	}
	streamOpenEndedChunkMB, _ := cmd.Flags().GetInt("stream-open-ended-chunk-mb")
	if cmd.Flags().Changed("stream-open-ended-chunk-mb") {
		os.Setenv("STREAM_OPEN_ENDED_CHUNK_MB", strconv.Itoa(streamOpenEndedChunkMB))
	}
	streamTimeoutSec, _ := cmd.Flags().GetInt("stream-timeout-sec")
	if streamTimeoutSec != 0 {
		os.Setenv("STREAM_TIMEOUT_SEC", strconv.Itoa(streamTimeoutSec))
	}
	streamMaxRetries, _ := cmd.Flags().GetInt("stream-max-retries")
	if streamMaxRetries != 0 {
		os.Setenv("STREAM_MAX_RETRIES", strconv.Itoa(streamMaxRetries))
	}
}

func (c *config) setupEnvVars(log *zap.Logger, cmd *cobra.Command) {
	c.loadFromEnvFile(log)
	c.loadConfigFromArgs(log, cmd)
	if strings.TrimSpace(os.Getenv("LOG_CHANNEL")) == "" {
		os.Setenv("LOG_CHANNEL", "0")
	}
	if strings.TrimSpace(os.Getenv("SUBTITLE_CHANNEL_ID")) == "" {
		os.Setenv("SUBTITLE_CHANNEL_ID", "0")
	}
	err := envconfig.Process("", c)
	if err != nil {
		log.Fatal("Error while parsing env variables", zap.Error(err))
	}
	var ipBlocked bool
	ip, err := getIP(c.UsePublicIP)
	if err != nil {
		log.Error("Error while getting IP", zap.Error(err))
		ipBlocked = true
	}
	if c.Host == "" {
		c.Host = "http://" + ip + ":" + strconv.Itoa(c.Port)
		if c.UsePublicIP {
			if ipBlocked {
				log.Sugar().Warn("Can't get public IP, using local IP")
			} else {
				log.Sugar().Warn("You are using a public IP, please be aware of the security risks while exposing your IP to the internet.")
				log.Sugar().Warn("Use 'HOST' variable to set a domain name")
			}
		}
		log.Sugar().Info("HOST not set, automatically set to " + c.Host)
	}
	val := reflect.ValueOf(c).Elem()
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "MULTI_TOKEN") {
			c.MultiTokens = append(c.MultiTokens, botTokenRegex.FindStringSubmatch(env)[1])
		}
	}
	val.FieldByName("MultiTokens").Set(reflect.ValueOf(c.MultiTokens))
}

func Load(log *zap.Logger, cmd *cobra.Command) {
	log = log.Named("Config")
	defer log.Info("Loaded config")
	ValueOf.setupEnvVars(log, cmd)
	if ValueOf.LogChannelID == 0 {
		log.Sugar().Info("LOG_CHANNEL is not set; legacy forwarding/hash routes are disabled unless you set LOG_CHANNEL")
	} else {
		ValueOf.LogChannelID = int64(stripInt(log, int(ValueOf.LogChannelID)))
	}
	if ValueOf.SubtitleChannelID == 0 {
		log.Sugar().Info("SUBTITLE_CHANNEL_ID is not set; subtitle docs must provide sourceChannel for subtitle routes to work")
	} else {
		ValueOf.SubtitleChannelID = int64(stripInt(log, int(ValueOf.SubtitleChannelID)))
	}
	if ValueOf.HashLength == 0 {
		log.Sugar().Info("HASH_LENGTH can't be 0, defaulting to 6")
		ValueOf.HashLength = 6
	}
	if ValueOf.HashLength > 32 {
		log.Sugar().Info("HASH_LENGTH can't be more than 32, changing to 32")
		ValueOf.HashLength = 32
	}
	if ValueOf.HashLength < 5 {
		log.Sugar().Info("HASH_LENGTH can't be less than 5, defaulting to 6")
		ValueOf.HashLength = 6
	}
	if ValueOf.StreamConcurrency <= 0 {
		log.Sugar().Info("STREAM_CONCURRENCY must be greater than 0, defaulting to 4")
		ValueOf.StreamConcurrency = 4
	}
	if ValueOf.StreamBufferCount <= 0 {
		log.Sugar().Info("STREAM_BUFFER_COUNT must be greater than 0, defaulting to 8")
		ValueOf.StreamBufferCount = 8
	}
	if ValueOf.StreamInitialBufferMB < 0 {
		log.Sugar().Info("STREAM_INITIAL_BUFFER_MB can't be negative, defaulting to 4")
		ValueOf.StreamInitialBufferMB = 4
	}
	if ValueOf.StreamOpenEndedChunkMB < 0 {
		log.Sugar().Info("STREAM_OPEN_ENDED_CHUNK_MB can't be negative, defaulting to 0")
		ValueOf.StreamOpenEndedChunkMB = 0
	}
	if ValueOf.StreamTimeoutSec <= 0 {
		log.Sugar().Info("STREAM_TIMEOUT_SEC must be greater than 0, defaulting to 30 seconds")
		ValueOf.StreamTimeoutSec = 30
	}
	if ValueOf.StreamMaxRetries <= 0 {
		log.Sugar().Info("STREAM_MAX_RETRIES must be greater than 0, defaulting to 3")
		ValueOf.StreamMaxRetries = 3
	}
	if ValueOf.StreamTokenTTLSec <= 0 {
		log.Sugar().Info("STREAM_TOKEN_TTL_SEC must be greater than 0, defaulting to 1800 seconds")
		ValueOf.StreamTokenTTLSec = 1800
	}
	if ValueOf.MongoURI == "" || ValueOf.MongoDB == "" {
		log.Sugar().Info("MongoDB stream routes are disabled; set MONGO_URI and MONGO_DB to enable DB-backed stream links")
	}
	if ValueOf.StreamSigningSecret == "" {
		log.Sugar().Info("Signed token routes are disabled; set STREAM_SIGNING_SECRET to enable DB-backed signed links")
	}
	if ValueOf.LinkSignAPIKey == "" {
		log.Sugar().Info("Link sign endpoint is disabled; set LINK_SIGN_API_KEY to enable protected token issuing")
	}
}

func getIP(public bool) (string, error) {
	var ip string
	var err error
	if public {
		ip, err = GetPublicIP()
	} else {
		ip, err = getInternalIP()
	}
	if ip == "" {
		ip = "localhost"
	}
	if err != nil {
		return "localhost", err
	}
	return ip, nil
}

// https://stackoverflow.com/a/23558495/15807350
func getInternalIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", errors.New("no internet connection")
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), nil
}

func GetPublicIP() (string, error) {
	resp, err := http.Get("https://api.ipify.org?format=text")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	ip, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if !checkIfIpAccessible(string(ip)) {
		return string(ip), errors.New("PORT is blocked by firewall")
	}
	return string(ip), nil
}

func checkIfIpAccessible(ip string) bool {
	conn, err := net.Dial("tcp", ip+":80")
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}

func stripInt(log *zap.Logger, a int) int {
	strA := strconv.Itoa(abs(a))
	lastDigits := strings.Replace(strA, "100", "", 1)
	result, err := strconv.Atoi(lastDigits)
	if err != nil {
		log.Sugar().Fatalln(err)
		return 0
	}
	return result
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
