package main

import (
	"log"

	aravis "github.com/hybridgroup/go-aravis"
)

func main() {
	aravis.UpdateDeviceList()

	numDev, err := aravis.GetNumDevices()
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Devices:", numDev)

	for i := range numDev {
		log.Println(aravis.GetDeviceId(i))
	}
}
