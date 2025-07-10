package main

import (
	"fmt"
	"log"

	aravis "github.com/hybridgroup/go-aravis"
)

func main() {
	// Update device list
	aravis.UpdateDeviceList()

	// Get number of devices
	numDev, err := aravis.GetNumDevices()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Found %d devices\n", numDev)

	// List detailed information for each device
	for i := range numDev {
		fmt.Printf("\n--- Device %d ---\n", i)

		// Get device ID
		deviceId, err := aravis.GetDeviceId(i)
		if err != nil {
			fmt.Printf("Error getting device ID: %v\n", err)
			continue
		}

		fmt.Printf("Device ID: %s\n", deviceId)

		// Create camera to get more detailed information
		camera, err := aravis.NewCamera(deviceId)
		if err != nil {
			fmt.Printf("Error creating camera: %v\n", err)
			continue
		}
		defer camera.Close()

		// Get vendor name
		vendor, err := camera.GetVendorName()
		if err == nil {
			fmt.Printf("Vendor: %s\n", vendor)
		} else {
			fmt.Printf("Vendor: Error - %v\n", err)
		}

		// Get model name
		model, err := camera.GetModelName()
		if err == nil {
			fmt.Printf("Model: %s\n", model)
		} else {
			fmt.Printf("Model: Error - %v\n", err)
		}

		// Get device serial number (NEW FUNCTION)
		serialNumber, err := camera.GetDeviceSerialNumber()
		if err == nil {
			fmt.Printf("Serial Number: %s\n", serialNumber)
		} else {
			fmt.Printf("Serial Number: Error - %v\n", err)
		}

		// Get sensor size
		width, height, err := camera.GetSensorSize()
		if err == nil {
			fmt.Printf("Sensor Size: %d x %d\n", width, height)
		} else {
			fmt.Printf("Sensor Size: Error - %v\n", err)
		}

		// Check if it's a GigE Vision device
		isGV, err := camera.IsGVDevice()
		if err == nil {
			fmt.Printf("GigE Vision Device: %t\n", isGV)
		} else {
			fmt.Printf("GigE Vision Device: Error - %v\n", err)
		}
	}

	if numDev == 0 {
		fmt.Println("No cameras found. Make sure cameras are connected and accessible.")
	}
}
