package main

import (
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"net/http"
	"time"

	aravis "github.com/MeKo-Christian/go-aravis"
)

var (
	exposureTime float64
	gain         float64
)

func serveJPEG(camera aravis.Camera) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		maxWidth, maxHeight, _ := camera.GetSensorSize()
		camera.SetRegion(0, 0, maxWidth, maxHeight)
		camera.SetExposureTimeAuto(aravis.AUTO_OFF)
		camera.SetExposureTime(exposureTime)
		camera.SetGain(gain)
		camera.SetFrameRate(3.75)
		camera.SetAcquisitionMode(aravis.ACQUISITION_MODE_SINGLE_FRAME)
		size, _ := camera.GetPayloadSize()
		_, _, width, height, _ := camera.GetRegion()

		// Create a stream
		stream, err := camera.CreateStream()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer stream.Close()

		// Add a buffer
		buffer, err := aravis.NewBuffer(size)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := stream.PushBuffer(buffer); err != nil {
			buffer.Close()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Start acquisition
		if err := camera.StartAcquisition(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer camera.StopAcquisition()

		buffer, err = stream.TimeoutPopBuffer(time.Second)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// The pop transfers the buffer back to us, and Stream.Close frees only
		// what is still in the stream's queues, so this handler used to leak a
		// whole frame per request on either of the two error paths below.
		defer buffer.Close()

		if s, _ := buffer.GetStatus(); s != aravis.BUFFER_STATUS_SUCCESS {
			http.Error(w, fmt.Sprintf("buffer not ready: status %d", s), http.StatusInternalServerError)
			return
		}

		data, err := buffer.GetData()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Image is in red-green bayer format
		img := aravis.NewBayerRG(
			image.Rectangle{image.Point{0, 0}, image.Point{width, height}},
		)
		img.Pix = data

		// Write JPEG to client
		jpeg.Encode(w, img, nil)
	})
}

func init() {
	flag.Float64Var(&exposureTime, "e", 10000, "Exposure time (in us)")
	flag.Float64Var(&gain, "g", 0, "Gain (in dB)")
}

func main() {
	var err error
	var numDevices uint

	flag.Parse()

	// Get devices
	aravis.UpdateDeviceList()

	if numDevices, err = aravis.GetNumDevices(); err != nil {
		log.Fatal(err)
	}

	// Must find at least one device
	if numDevices == 0 {
		log.Fatal("No devices found. Exiting.")
		return
	}

	for index := range numDevices {
		name, _ := aravis.GetDeviceId(index)

		camera, _ := aravis.NewCamera(name)
		defer camera.Close()

		http.Handle(fmt.Sprintf("/%d.jpg", index), serveJPEG(camera))
	}

	log.Fatal(http.ListenAndServe(":8000", nil))
}
