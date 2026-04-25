package config

import (
	"os"
	"time"
)

type Config struct {
	Addr                string
	Env                 string
	DefaultLang         string
	DatabaseURL         string
	SessionTTL          time.Duration
	TelegramBotUsername string
	BotServiceToken     string
}

func Load() Config {
	return Config{
		Addr:                getEnv("APP_ADDR", ":8080"),
		Env:                 getEnv("APP_ENV", "development"),
		DefaultLang:         getEnv("APP_DEFAULT_LANG", "ru"),
		DatabaseURL:         getEnv("APP_DATABASE_URL", "postgres://notifier:notifier@localhost:5432/notifier?sslmode=disable"),
		SessionTTL:          getDuration("APP_SESSION_TTL", 30*24*time.Hour),
		TelegramBotUsername: getEnv("APP_TELEGRAM_BOT_USERNAME", "your_bot_username"),
		BotServiceToken:     getEnv("APP_BOT_SERVICE_TOKEN", "change_me_bot_service_token"),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		duration, err := time.ParseDuration(value)
		if err == nil {
			return duration
		}
	}

	return fallback
}
