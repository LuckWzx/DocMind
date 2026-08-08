package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

var globalConfig *Config

// Load 加载配置文件
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// 设置配置文件路径
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("./configs")
		v.AddConfigPath(".")
	}

	// 环境变量前缀
	v.SetEnvPrefix("CLOUDQUE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 解析配置
	config := &Config{}
	if err := v.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// SSE 护栏默认值兜底：未配置 max_body_bytes 时若为 0，
	// http.MaxBytesReader 会拒绝一切请求体（HTTP 413），默认放宽到 10MB
	if config.SSE.MaxBodyBytes <= 0 {
		config.SSE.MaxBodyBytes = 10 << 20
	}

	// 从环境变量覆盖敏感配置
	if val := os.Getenv("POSTGRES_PASSWORD"); val != "" {
		config.Database.PostgreSQL.Password = val
	}
	if val := os.Getenv("MYSQL_PASSWORD"); val != "" {
		config.Database.MySQL.Password = val
	}
	if val := os.Getenv("REDIS_PASSWORD"); val != "" {
		config.Database.Redis.Password = val
	}
	if val := os.Getenv("JWT_SECRET"); val != "" {
		config.JWT.Secret = val
	}
	if val := os.Getenv("DOCREADER_ADDR"); val != "" {
		config.DocReader.Addr = val
	}
	if val := os.Getenv("STORAGE_LOCAL_ROOT"); val != "" {
		config.Storage.LocalRoot = val
	}
	if val := os.Getenv("MINIO_ENDPOINT"); val != "" {
		config.MinIO.Endpoint = val
	}
	if val := os.Getenv("MINIO_BASE_URL"); val != "" {
		config.MinIO.BaseURL = val
	}
	if val := os.Getenv("MINIO_ACCESS_KEY_ID"); val != "" {
		config.MinIO.AccessKeyID = val
	}
	if val := os.Getenv("MINIO_ACCESS_KEY_SECRET"); val != "" {
		config.MinIO.AccessKeySecret = val
	}
	if val := os.Getenv("MINIO_BUCKET_NAME"); val != "" {
		config.MinIO.BucketName = val
	}
	if val := os.Getenv("MINIO_PATH_PREFIX"); val != "" {
		config.MinIO.PathPrefix = val
	}
	if val := os.Getenv("MINIO_USE_SSL"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			config.MinIO.UseSSL = parsed
		}
	}

	globalConfig = config
	return config, nil
}

// Get 获取全局配置
func Get() *Config {
	if globalConfig == nil {
		panic("配置未初始化，请先调用 Load() 加载配置")
	}
	return globalConfig
}

// MustLoad 加载配置，失败时 panic
func MustLoad(configPath string) *Config {
	config, err := Load(configPath)
	if err != nil {
		panic(err)
	}
	return config
}
