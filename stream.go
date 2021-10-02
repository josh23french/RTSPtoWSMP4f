package main

import (
	"errors"
	"time"

	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/format/rtspv2"
	"github.com/rs/zerolog/log"
)

var (
	ErrorStreamExitNoVideoOnStream = errors.New("Stream Exit No Video On Stream")
	ErrorStreamExitRtspDisconnect  = errors.New("Stream Exit Rtsp Disconnect")
	ErrorStreamExitNoViewer        = errors.New("Stream Exit On Demand No Viewer")
)

func serveStreams() {
	for k, v := range Config.Streams {
		if !v.OnDemand {
			go RTSPWorkerLoop(k, v.URL, v.OnDemand)
		}
	}
}

func RTSPWorkerLoop(name, url string, OnDemand bool) {
	defer Config.RunUnlock(name)
	for {
		log.Info().Msgf("%v Stream Try Connect", name)
		err := RTSPWorker(name, url, OnDemand)
		if err != nil {
			log.Error().Err(err).Msg("Error running RTSPWorker")
		}
		if OnDemand && !Config.HasViewer(name) {
			log.Error().Err(ErrorStreamExitNoViewer).Msgf("No viewer for %v; exiting", name)
			return
		}
		time.Sleep(1 * time.Second)
	}
}

func RTSPWorker(name, url string, OnDemand bool) error {
	keyTest := time.NewTimer(20 * time.Second)
	clientTest := time.NewTimer(20 * time.Second)
	RTSPClient, err := rtspv2.Dial(rtspv2.RTSPClientOptions{URL: url, DisableAudio: true, DialTimeout: 3 * time.Second, ReadWriteTimeout: 3 * time.Second, Debug: false})
	if err != nil {
		return err
	}
	defer RTSPClient.Close()
	if RTSPClient.CodecData != nil {
		Config.coAd(name, RTSPClient.CodecData)
	}
	var AudioOnly bool
	if len(RTSPClient.CodecData) == 1 && RTSPClient.CodecData[0].Type().IsAudio() {
		AudioOnly = true
	}
	//MuxerNVR := nvr.NewMuxer(RTSPClient.CodecData, name, "nvr", 1*time.Second)
	//defer MuxerNVR.Close()
	for {
		select {
		case <-clientTest.C:
			if OnDemand && !Config.HasViewer(name) {
				return ErrorStreamExitNoViewer
			}
		case <-keyTest.C:
			return ErrorStreamExitNoVideoOnStream
		case signals := <-RTSPClient.Signals:
			switch signals {
			case rtspv2.SignalCodecUpdate:
				Config.coAd(name, RTSPClient.CodecData)
				//MuxerNVR.CodecUpdate(RTSPClient.CodecData)
			case rtspv2.SignalStreamRTPStop:
				return ErrorStreamExitRtspDisconnect
			}
		case packetAV := <-RTSPClient.OutgoingPacketQueue:
			if AudioOnly || packetAV.IsKeyFrame {
				keyTest.Reset(20 * time.Second)
			}
			if packetAV.IsKeyFrame {
				grabScreenshot(name, *packetAV)
			}
			//MuxerNVR.WritePacket(packetAV)
			Config.cast(name, *packetAV)
		}
	}
}

func grabScreenshot(name string, pkt av.Packet) {
	codecs := Config.codecGet(name)
	var codec av.CodecData
	for _, codec = range codecs {
		if codec.Type() == av.H264 {
			break
		}
	}
	if codec == nil {
		log.Error().Msg("No H264 codecs")
		return
	}

	// decoder, err := ffmpeg.NewVideoDecoder(codec)
	// if err != nil {
	// 	log.Error().Err(err).Msg("VideoDecoder Error")
	// 	return
	// }
	// frame, err := decoder.Decode(pkt.Data)
	// if err != nil {
	// 	log.Error().Err(err).Msg("Decode Error")
	// 	return
	// }
	// buffer := new(bytes.Buffer)

	// b := frame.Image.Bounds()
	// log.Info().Msgf("image -----\nbounds: %v\nx: %v\n\ny: %v", b, frame.Image.CStride, frame.Image.YStride)
	return
	// m := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	// draw.Draw(m, m.Bounds(), &frame.Image, b.Min, draw.Src)

	// err = jpeg.Encode(buffer, m, &jpeg.Options{
	// 	Quality: 90,
	// })
	// if err != nil {
	// 	log.Error().Err(err).Msg("JPEG encode Error")
	// 	return
	// }
	// Config.screenshotSet(name, buffer)
}
