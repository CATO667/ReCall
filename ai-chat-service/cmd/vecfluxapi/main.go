package main

import (
	"ai-chat-service/chat-server/rerank"
	chatserver "ai-chat-service/chat-server/server"
	"ai-chat-service/chat-server/storage"
	"ai-chat-service/chat-server/vector"
	"ai-chat-service/interceptor"
	"ai-chat-service/pkg/config"
	"ai-chat-service/pkg/db/kvstore"
	"ai-chat-service/pkg/log"
	"ai-chat-service/vecfluxapi/orchestrator"
	vpb "ai-chat-service/vecfluxapi/proto"
	vserver "ai-chat-service/vecfluxapi/server"
	"flag"
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

var configFile = flag.String("config", "vecfluxapi.dev.config.yaml", "")

func main() {
	flag.Parse()
	config.InitConfig(*configFile)
	cnf := config.GetConfig()

	log.SetLevel(cnf.Log.Level)
	log.SetOutput(log.GetRotateWriter(cnf.Log.LogPath))
	log.SetPrintCaller(true)

	logger := log.NewLogger()
	logger.SetLevel(cnf.Log.Level)
	logger.SetOutput(log.GetRotateWriter(cnf.Log.LogPath))
	logger.SetPrintCaller(true)

	kvstore.InitPools(cnf)
	qaStore := storage.NewQAStore()
	topicStore := storage.NewTopicStore()
	embedder, vectorStore := vector.NewFromConfig(cnf, qaStore)
	rerankService := rerank.NewServiceFromConfig(cnf)
	orch := orchestrator.New(cnf, logger, qaStore, embedder, vectorStore, rerankService)
	topicClassifier := chatserver.NewTopicClassifier(qaStore, topicStore, vectorStore, cnf)

	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cnf.Server.IP, cnf.Server.Port))
	if err != nil {
		log.Fatal(err)
	}

	s := grpc.NewServer(
		grpc.UnaryInterceptor(interceptor.UnaryAuthInterceptor),
		grpc.StreamInterceptor(interceptor.StreamAuthInterceptor),
	)
	api := vserver.New(cnf, logger, qaStore, embedder, vectorStore, orch, topicClassifier)
	vpb.RegisterVecFluxAPIServer(s, api)

	healthCheckSrv := health.NewServer()
	grpc_health_v1.RegisterHealthServer(s, healthCheckSrv)

	logger.InfoF("vecfluxapi listening on %s:%d", cnf.Server.IP, cnf.Server.Port)
	if err = s.Serve(lis); err != nil {
		log.Fatal(err)
	}
}
