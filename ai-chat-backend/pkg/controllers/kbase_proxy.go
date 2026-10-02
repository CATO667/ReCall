package controllers

import (
	"ai-chat-backend/pkg/config"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

func RegisterKBaseProxy(r *gin.Engine, cnf *config.Config) {
	target := cnf.DependOn.KBaseAPI.Address
	if target == "" {
		target = "http://127.0.0.1:50056"
	}
	remote, err := url.Parse(strings.TrimRight(target, "/"))
	if err != nil {
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(remote)
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = remote.Scheme
		req.URL.Host = remote.Host
		req.Host = remote.Host
	}
	r.Any("/api/kbase/*path", func(c *gin.Context) {
		proxy.ServeHTTP(c.Writer, c.Request)
	})
}
