package main

import (
	"fmt"
	"log"
	"time"

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

	if numDev == 0 {
		fmt.Println("No cameras found. This example demonstrates advanced buffer features.")
		fmt.Println("Connect a camera to test multipart and chunk data capabilities.")

		return
	}

	fmt.Printf("Found %d device(s)\n", numDev)

	// Use the first device
	deviceId, err := aravis.GetDeviceId(0)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Using device: %s\n", deviceId)

	// Create camera
	camera, err := aravis.NewCamera(deviceId)
	if err != nil {
		log.Fatal(err)
	}
	defer camera.Close()

	// Get camera info
	vendor, err := camera.GetVendorName()
	if err == nil {
		fmt.Printf("Camera: %s ", vendor)
	}

	model, err := camera.GetModelName()
	if err == nil {
		fmt.Printf("%s ", model)
	}

	serial, err := camera.GetDeviceSerialNumber()
	if err == nil {
		fmt.Printf("(S/N: %s)", serial)
	}

	fmt.Println()

	// Configure camera for single frame acquisition
	camera.SetAcquisitionMode(aravis.ACQUISITION_MODE_SINGLE_FRAME)

	// Get payload size
	payloadSize, err := camera.GetPayloadSize()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Payload size: %d bytes\n", payloadSize)

	// Create stream
	stream, err := camera.CreateStream()
	if err != nil {
		log.Fatal(err)
	}
	defer stream.Close()

	// Create buffer
	buffer, err := aravis.NewBuffer(payloadSize)
	if err != nil {
		log.Fatal(err)
	}

	// Add buffer to stream
	stream.PushBuffer(buffer)

	// Start acquisition
	err = camera.StartAcquisition()
	if err != nil {
		log.Fatal(err)
	}
	defer camera.StopAcquisition()

	// Get buffer with timeout
	fmt.Println("Acquiring frame...")

	buffer, err = stream.TimeoutPopBuffer(5 * time.Second)
	if err != nil {
		log.Fatal(err)
	}

	// Check buffer status
	status, err := buffer.GetStatus()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Buffer status: %d\n", status)

	if status != aravis.BUFFER_STATUS_SUCCESS {
		fmt.Printf("Buffer acquisition failed with status: %d\n", status)
		return
	}

	// === MULTIPART BUFFER ANALYSIS ===
	fmt.Println("\n=== Multipart Buffer Analysis ===")

	// Check number of parts
	numParts, err := buffer.GetNumParts()
	if err != nil {
		fmt.Printf("Error getting number of parts: %v\n", err)
	} else {
		fmt.Printf("Number of parts: %d\n", numParts)

		// Analyze each part
		for index := range numParts {
			fmt.Printf("\n--- Part %d ---\n", index)

			// Get part data
			partData, err := buffer.GetPartData(index)
			if err != nil {
				fmt.Printf("Error getting part %d data: %v\n", index, err)
			} else {
				fmt.Printf("Part %d data size: %d bytes\n", index, len(partData))
			}

			// Get part component ID
			componentId, err := buffer.GetPartComponentId(index)
			if err != nil {
				fmt.Printf("Error getting part %d component ID: %v\n", index, err)
			} else {
				fmt.Printf("Part %d component ID: %d\n", index, componentId)
			}

			// Get part data type
			dataType, err := buffer.GetPartDataType(index)
			if err != nil {
				fmt.Printf("Error getting part %d data type: %v\n", index, err)
			} else {
				fmt.Printf("Part %d data type: %d\n", index, dataType)
			}

			// Get part pixel format
			pixelFormat, err := buffer.GetPartPixelFormat(index)
			if err != nil {
				fmt.Printf("Error getting part %d pixel format: %v\n", index, err)
			} else {
				fmt.Printf("Part %d pixel format: 0x%x\n", index, pixelFormat)
			}

			// Get part dimensions and position
			width, err := buffer.GetPartWidth(index)
			if err != nil {
				fmt.Printf("Error getting part %d width: %v\n", index, err)
			} else {
				fmt.Printf("Part %d width: %d\n", index, width)
			}

			height, err := buffer.GetPartHeight(index)
			if err != nil {
				fmt.Printf("Error getting part %d height: %v\n", index, err)
			} else {
				fmt.Printf("Part %d height: %d\n", index, height)
			}

			x, err := buffer.GetPartX(index)
			if err != nil {
				fmt.Printf("Error getting part %d x: %v\n", index, err)
			} else {
				fmt.Printf("Part %d x position: %d\n", index, x)
			}

			y, err := buffer.GetPartY(index)
			if err != nil {
				fmt.Printf("Error getting part %d y: %v\n", index, err)
			} else {
				fmt.Printf("Part %d y position: %d\n", index, y)
			}
		}

		// Test finding component
		if numParts > 0 {
			// Try to find the first component
			componentId, _ := buffer.GetPartComponentId(0)

			partIndex, err := buffer.FindComponent(componentId)
			if err != nil {
				fmt.Printf("Error finding component %d: %v\n", componentId, err)
			} else {
				fmt.Printf("Component %d found at part index: %d\n", componentId, partIndex)
			}
		}
	}

	// === CHUNK DATA ANALYSIS ===
	fmt.Println("\n=== Chunk Data Analysis ===")

	// Check if buffer has chunks
	hasChunks := buffer.HasChunks()
	fmt.Printf("Has chunks: %t\n", hasChunks)

	if hasChunks {
		fmt.Println("Buffer contains chunk data!")
		// Note: GetChunkData() function is not available in current library version
		// but HasChunks() can be used to detect chunk presence
	}

	// === STANDARD BUFFER DATA ===
	fmt.Println("\n=== Standard Buffer Data ===")

	// Get standard buffer data
	data, err := buffer.GetData()
	if err != nil {
		fmt.Printf("Error getting buffer data: %v\n", err)
	} else {
		fmt.Printf("Buffer data size: %d bytes\n", len(data))

		if len(data) > 0 {
			fmt.Printf("First 16 bytes: %x\n", data[:minInt(16, len(data))])
		}
	}

	fmt.Println("\nAdvanced buffer analysis complete!")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}

	return b
}
