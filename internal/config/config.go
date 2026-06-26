package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	ServerPort        string
	ServerDomain      string
	DBHost            string
	DBPort            string
	DBUser            string
	DBPassword        string
	DBName            string
	JWTSecret         string
	JWTRefreshSecret  string
	MediaStoragePath  string
	MaxUploadBytes    int64
	LiveKitURL        string
	LiveKitAPIKey     string
	LiveKitAPISecret  string
	VAPIDPublicKey    string
	VAPIDPrivateKey   string
}

func LoadConfig() *Config {
	// Load .env file if it exists (for local development)
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	maxUploadBytes, err := strconv.ParseInt(getEnv("MAX_UPLOAD_BYTES", "2147483648"), 10, 64)
	if err != nil {
		maxUploadBytes = 2147483648 // Default: 2 GB
	}

	return &Config{
		ServerPort:        getEnv("SERVER_PORT", "8080"),
		ServerDomain:      getEnv("SERVER_DOMAIN", "localhost"),
		DBHost:            getEnv("DB_HOST", "localhost"),
		DBPort:            getEnv("DB_PORT", "5432"),
		DBUser:            getEnv("DB_USER", "nit"),
		DBPassword:        getEnv("DB_PASSWORD", "nitpass"),
		DBName:            getEnv("DB_NAME", "nit"),
		JWTSecret:         getEnv("JWT_SECRET", "supersecretjwtkey"),
		JWTRefreshSecret:  getEnv("JWT_REFRESH_SECRET", "supersecretjwtrefreshkey"),
		MediaStoragePath:  getEnv("MEDIA_STORAGE_PATH", "./uploads"),
		MaxUploadBytes:    maxUploadBytes,
		LiveKitURL:        getEnv("LIVEKIT_URL", "http://localhost:7880"),
		LiveKitAPIKey:     getEnv("LIVEKIT_API_KEY", "devkey"),
		LiveKitAPISecret:  getEnv("LIVEKIT_API_SECRET", "secret"),
		VAPIDPublicKey:    getEnv("VAPID_PUBLIC_KEY", ""),
		VAPIDPrivateKey:   getEnv("VAPID_PRIVATE_KEY", ""),
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
