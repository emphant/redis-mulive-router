package proxy

import (
	"net/http"
	"strings"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/gzip"
	"github.com/martini-contrib/render"
	"github.com/emphant/redis-mulive-router/pkg/utils/log"
	"github.com/emphant/redis-mulive-router/pkg/utils/rpc"
)

type apiServer struct {
	proxy *Proxy
}

func newApiServer(p *Proxy) http.Handler {
	m := martini.New()
	m.Use(martini.Recovery())
	m.Use(render.Renderer())
	m.Use(func(w http.ResponseWriter, req *http.Request, c martini.Context) {
		path := req.URL.Path
		if req.Method != "GET" && strings.HasPrefix(path, "/api/") {
			var remoteAddr = req.RemoteAddr
			var headerAddr string
			for _, key := range []string{"X-Real-IP", "X-Forwarded-For"} {
				if val := req.Header.Get(key); val != "" {
					headerAddr = val
					break
				}
			}
			log.Warnf("[%p] API call %s from %s [%s]", p, path, remoteAddr, headerAddr)
		}
		c.Next()
	})
	m.Use(gzip.All())
	m.Use(func(c martini.Context, w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	})

	api := &apiServer{proxy: p}

	r := martini.NewRouter()
	r.Get("/", func(r render.Render) {
		r.Redirect("/proxy")
	})
	r.Any("/debug/**", func(w http.ResponseWriter, req *http.Request) {
		http.DefaultServeMux.ServeHTTP(w, req)
	})

	r.Group("/proxy", func(r martini.Router) {
		r.Get("", api.Overview)
		r.Get("/model", api.Model)
		r.Get("/stats", api.StatsNoXAuth)
		//r.Get("/slots", api.SlotsNoXAuth)
	})
	//r.Group("/api/proxy", func(r martini.Router) {
	//	r.Get("/model", api.Model)
	//	r.Get("/xping/:xauth", api.XPing)
	//	r.Get("/stats/:xauth", api.Stats)
	//	r.Get("/stats/:xauth/:flags", api.Stats)
	//	r.Get("/slots/:xauth", api.Slots)
	//	r.Put("/start/:xauth", api.Start)
	//	r.Put("/stats/reset/:xauth", api.ResetStats)
	//	r.Put("/forcegc/:xauth", api.ForceGC)
	//	r.Put("/shutdown/:xauth", api.Shutdown)
	//	r.Put("/loglevel/:xauth/:value", api.LogLevel)
	//	r.Put("/fillslots/:xauth", binding.Json([]*models.Slot{}), api.FillSlots)
	//	r.Put("/sentinels/:xauth", binding.Json(models.Sentinel{}), api.SetSentinels)
	//	r.Put("/sentinels/:xauth/rewatch", api.RewatchSentinels)
	//})

	m.MapTo(r, (*martini.Routes)(nil))
	m.Action(r.Handle)
	return m
}

func (s *apiServer) Overview() (int, string) {
	return rpc.ApiResponseJson(s.proxy.Overview(StatsFull))
}

func (s *apiServer) Model() (int, string) {
	return rpc.ApiResponseJson(s.proxy.Model())
}

func (s *apiServer) StatsNoXAuth() (int, string) {
	return rpc.ApiResponseJson(s.proxy.Stats(StatsFull))
}