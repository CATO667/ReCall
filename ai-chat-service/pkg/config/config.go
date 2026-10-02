package config

import (
	"github.com/spf13/viper"
	"log"
)

type Config struct {
	Server struct {
		IP          string
		Port        int
		AccessToken string
	}
	Log struct {
		Level   string
		LogPath string `mapstructure:"logPath"`
	} `mapstructure:"log"`
	Chat struct {
		ApiKey            string  `mapstructure:"api_key"`
		BaseUrl           string  `mapstructure:"base_url"`
		Model             string  `mapstructure:"model"`
		MaxTokens         int     `mapstructure:"max_tokens"`
		Temperature       float32 `mapstructure:"temperature"`
		TopP              float32 `mapstructure:"top_p"`
		PresencePenalty   float32 `mapstructure:"presence_penalty"`
		FrequencyPenalty  float32 `mapstructure:"frequency_penalty"`
		BotDesc           string  `mapstructure:"bot_desc"`
		MinResponseTokens int     `mapstructure:"min_response_tokens"`
		ContextTTL        int     `mapstructure:"context_ttl"`
		ContextLen        int     `mapstructure:"context_len"`
		QACacheTTL        int     `mapstructure:"qa_cache_ttl"`
	}
	KVStore struct {
		WriteHost string `mapstructure:"write_host"`
		ReadHost  string `mapstructure:"read_host"`
		Port      int    `mapstructure:"port"`
		ReadPort  int    `mapstructure:"read_port"`
		Prefix    string `mapstructure:"prefix"`
	}
	Embedder struct {
		Provider string `mapstructure:"provider"`
		ApiKey   string `mapstructure:"api_key"`
		BaseUrl  string `mapstructure:"base_url"`
		Model    string `mapstructure:"model"`
	}
	Vector struct {
		Enabled             bool    `mapstructure:"enabled"`
		Provider            string  `mapstructure:"provider"`
		Address             string  `mapstructure:"address"`
		WriteAddress        string  `mapstructure:"write_address"`
		ReadAddress         string  `mapstructure:"read_address"`
		Collection          string  `mapstructure:"collection"`
		IndexType           string  `mapstructure:"index_type"`
		VectorDim           int     `mapstructure:"vector_dim"`
		RecallTopK          int     `mapstructure:"recall_top_k"`
		SimilarityThreshold       float32 `mapstructure:"similarity_threshold"`
		CacheSimilarityThreshold  float32 `mapstructure:"cache_similarity_threshold"`
		TopK                      int     `mapstructure:"top_k"`
		QAScoreBoost              float32 `mapstructure:"qa_score_boost"`
		TopicSimilarityThreshold  float32 `mapstructure:"topic_similarity_threshold"`
		OrchestrationV2           bool    `mapstructure:"orchestration_v2"`
		MilvusAddress       string  `mapstructure:"milvus_address"`
		Url                 string
		Username            string
		Pwd                 string
		Database            string
		Timeout             int
		MaxIdleConnPerHost  int
		ReadConsistency     string
		IdleConnTimeout     int
	}
	Rerank struct {
		Enabled      bool    `mapstructure:"enabled"`
		WeightVector  float32 `mapstructure:"weight_vector"`
		WeightLexical float32 `mapstructure:"weight_lexical"`
		WeightTopic   float32 `mapstructure:"weight_topic"`
	}
	KBase struct {
		Enabled          bool   `mapstructure:"enabled"`
		Collection       string `mapstructure:"collection"`
		ChunkStrategy    string `mapstructure:"chunk_strategy"`
		ChunkSize        int    `mapstructure:"chunk_size"`
		ChunkOverlap     int    `mapstructure:"chunk_overlap"`
		MinChunkSize     int     `mapstructure:"min_chunk_size"`
		EmbedConcurrency int     `mapstructure:"embed_concurrency"`
		SimilarityThreshold float32 `mapstructure:"similarity_threshold"`
		HybridEnabled       bool    `mapstructure:"hybrid_enabled"`
		HybridVectorWeight  float32 `mapstructure:"hybrid_vector_weight"`
		HybridKeywordWeight float32 `mapstructure:"hybrid_keyword_weight"`
		RerankEnabled       bool    `mapstructure:"rerank_enabled"`
		RerankKeywordBoost  float32 `mapstructure:"rerank_keyword_boost"`
		HTTPAPIEnabled      bool    `mapstructure:"http_api_enabled"`
		HTTPAPIPort         int     `mapstructure:"http_api_port"`
	}
	DependOn struct {
		Sensitive struct {
			Address     string
			AccessToken string
		}
		Keywords struct {
			Address     string
			AccessToken string
		}
		Tokenizer struct {
			Address string
		}
		Vecfluxapi struct {
			Address     string
			AccessToken string
		} `mapstructure:"vecfluxapi"`
	}
}

var conf *Config

func InitConfig(filePath string, typ ...string) {
	v := viper.New()
	v.SetConfigFile(filePath)
	if len(typ) > 0 {
		v.SetConfigType(typ[0])
	}
	err := v.ReadInConfig()
	if err != nil {
		log.Fatal(err)
	}
	conf = &Config{}
	err = v.Unmarshal(conf)
	if err != nil {
		log.Fatal(err)
	}
}

func GetConfig() *Config {
	return conf
}
