package main

import (
	"log"

	aravis "github.com/MeKo-Christian/go-aravis"
)

func main() {
	aravis.UpdateDeviceList()

	numDev := aravis.GetNumDevices()

	log.Println("Devices:", numDev)

	for i := range numDev {
		log.Println(aravis.GetDeviceId(i))
	}
}
