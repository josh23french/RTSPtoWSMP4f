package main

import (
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/format/mp4f"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"golang.org/x/net/websocket"
)

func serveHTTP() {
	router := gin.Default()
	gin.SetMode(gin.DebugMode)
	router.LoadHTMLGlob("web/templates/*")
	router.GET("/", func(c *gin.Context) {
		fi, all := Config.list()
		sort.Strings(all)
		c.HTML(http.StatusOK, "index.tmpl", gin.H{
			"port":     Config.Server.HTTPPort,
			"suuid":    fi,
			"suuidMap": all,
			"version":  time.Now().String(),
		})
	})
	router.GET("/player/:suuid", func(c *gin.Context) {
		_, all := Config.list()
		sort.Strings(all)
		c.HTML(http.StatusOK, "index.tmpl", gin.H{
			"port":     Config.Server.HTTPPort,
			"suuid":    c.Param("suuid"),
			"suuidMap": all,
			"version":  time.Now().String(),
		})
	})
	router.GET("/screenshot/:size/:suuid", func(c *gin.Context) {
		size := c.Param("size")
		if size != "low" && size != "high" {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		suuid := c.Param("suuid")
		log.Debug().Msgf("screenshot(%v, %v)", size, suuid)
		screenshot := Config.screenshotGet(suuid)

		c.Writer.Header().Set("Content-Type", "image/jpeg")
		c.Writer.Header().Set("Content-Length", strconv.Itoa(len(screenshot.Bytes())))
		if _, err := c.Writer.Write(screenshot.Bytes()); err != nil {
			log.Error().Err(err).Msg("unable to write image.")
		}
		return
	})
	router.GET("/ws/:suuid", func(c *gin.Context) {
		handler := websocket.Handler(ws)
		handler.ServeHTTP(c.Writer, c.Request)
	})
	router.StaticFS("/static", http.Dir("web/static"))
	err := router.Run(Config.Server.HTTPPort)
	if err != nil {
		log.Fatal().Err(err).Msg("Error running server")
	}
}
func ws(ws *websocket.Conn) {
	defer ws.Close()
	suuid := ws.Request().FormValue("suuid")
	log.Info().Msgf("Request: %v", suuid)
	if !Config.exists(suuid) {
		log.Warn().Msgf("Stream Not Found: %v", suuid)
		return
	}
	Config.RunIFNotRun(suuid)
	ws.SetWriteDeadline(time.Now().Add(5 * time.Second))
	cuuid, ch := Config.clAd(suuid)
	defer Config.clientDelete(suuid, cuuid)
	codecs := Config.codecGet(suuid)
	if codecs == nil {
		log.Error().Msg("Codecs Error")
		return
	}
	for i, codec := range codecs {
		if codec.Type().IsAudio() && codec.Type() != av.AAC {
			log.Info().Msgf("Track %i Audio Codec Work Only AAC", i)
		}
	}
	muxer := mp4f.NewMuxer(nil)
	err := muxer.WriteHeader(codecs)
	if err != nil {
		log.Error().Err(err).Msg("muxer.WriteHeader")
		return
	}
	meta, init := muxer.GetInit(codecs)
	err = websocket.Message.Send(ws, append([]byte{9}, meta...))
	if err != nil {
		log.Error().Err(err).Msg("websocket.Message.Send")
		return
	}
	err = websocket.Message.Send(ws, init)
	if err != nil {
		return
	}
	var start bool
	go func() {
		for {
			var message string
			err := websocket.Message.Receive(ws, &message)
			if err != nil {
				ws.Close()
				return
			}
		}
	}()
	noVideo := time.NewTimer(10 * time.Second)
	var timeLine = make(map[int8]time.Duration)
	for {
		select {
		case <-noVideo.C:
			log.Debug().Msg("noVideo")
			return
		case pck := <-ch:
			if pck.IsKeyFrame {
				noVideo.Reset(10 * time.Second)
				start = true
			}
			if !start {
				continue
			}
			timeLine[pck.Idx] += pck.Duration
			pck.Time = timeLine[pck.Idx]
			ready, buf, _ := muxer.WritePacket(pck, false)
			if ready {
				err = ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err != nil {
					return
				}
				err := websocket.Message.Send(ws, buf)
				if err != nil {
					return
				}
			}
		}
	}
}
