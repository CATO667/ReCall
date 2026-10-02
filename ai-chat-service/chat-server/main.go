package main

import (
	metrics_app "ai-chat-service/chat-server/metrics-app"
	metrics_bus "ai-chat-service/chat-server/metrics-bus"
	"ai-chat-service/chat-server/httpapi"
	"ai-chat-service/chat-server/rerank"
	"ai-chat-service/chat-server/server"
	"ai-chat-service/chat-server/storage"
	"ai-chat-service/chat-server/vector"
	"ai-chat-service/interceptor"
	"ai-chat-service/pkg/config"
	"ai-chat-service/pkg/db/kvstore"
	"ai-chat-service/pkg/log"
	"ai-chat-service/proto"
	vfclient "ai-chat-service/vecfluxapi/client"
	"flag"
	"fmt"
	"context"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"net"
	"net/http"
)

var (
	configFile = flag.String("config", "dev.config.yaml", "")
)

func main() {
	flag.Parse()
	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	busMetrics := metrics_bus.NewBusMetrics(registry)

	http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	go http.ListenAndServe(":8080", nil)

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
	topicClassifier := server.NewTopicClassifier(qaStore, topicStore, vectorStore, cnf)
	kbaseHTTP := httpapi.NewServer(cnf, logger, qaStore, topicStore)
	go kbaseHTTP.Start(context.Background())

	var vfClient *vfclient.Client
	if cnf.DependOn.Vecfluxapi.Address != "" {
		var vfErr error
		vfClient, vfErr = vfclient.New(cnf.DependOn.Vecfluxapi.Address, cnf.DependOn.Vecfluxapi.AccessToken)
		if vfErr != nil {
			log.Fatal(vfErr)
		}
		logger.InfoF("vecfluxapi client enabled: %s", cnf.DependOn.Vecfluxapi.Address)
	}

	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cnf.Server.IP, cnf.Server.Port))
	if err != nil {
		log.Fatal(err)
	}
	s := grpc.NewServer(grpc.UnaryInterceptor(interceptor.UnaryAuthInterceptor), grpc.StreamInterceptor(metrics_app.NewStreamMiddleware(registry).WrapHandler()))
	service := server.NewChatService(qaStore, topicClassifier, embedder, vectorStore, rerankService, cnf, logger, busMetrics, vfClient)
	proto.RegisterChatServer(s, service)

	healthCheckSrv := health.NewServer()
	grpc_health_v1.RegisterHealthServer(s, healthCheckSrv)

	if err = s.Serve(lis); err != nil {
		log.Fatal(err)
	}
}
