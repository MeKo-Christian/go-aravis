package main

import (
	"fmt"
	"log"

	aravis "github.com/hybridgroup/go-aravis"
)

func main() {
	fmt.Println("=== Advanced Register/Memory Access Example ===")
	fmt.Println("WARNING: This example demonstrates low-level register access.")
	fmt.Println("Use with caution as incorrect register access can damage cameras!")
	fmt.Println()

	// Update device list
	aravis.UpdateDeviceList()

	// Get number of devices
	numDev, err := aravis.GetNumDevices()
	if err != nil {
		log.Fatal(err)
	}

	if numDev == 0 {
		fmt.Println("No cameras found. This example demonstrates register/memory access.")
		fmt.Println("Connect a camera to test low-level register operations.")

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

	// Get device
	device, err := camera.GetDevice()
	if err != nil {
		log.Fatal(err)
	}

	// Get camera info
	if vendor, err := camera.GetVendorName(); err == nil {
		fmt.Printf("Camera: %s ", vendor)
	}

	if model, err := camera.GetModelName(); err == nil {
		fmt.Printf("%s ", model)
	}

	if serial, err := camera.GetDeviceSerialNumber(); err == nil {
		fmt.Printf("(S/N: %s)", serial)
	}

	fmt.Println()

	// Check if this is a GigE Vision device (register access is more common with GigE)
	if isGV, err := camera.IsGVDevice(); err == nil && isGV {
		fmt.Println("This is a GigE Vision device - register access available")
	} else {
		fmt.Println("This may not be a GigE Vision device - register access may be limited")
	}

	fmt.Println("\n=== Register Access Examples ===")

	// Example 1: Read some common GigE Vision bootstrap registers
	// Note: These are standard GigE Vision registers, but cameras may behave differently
	commonRegisters := map[string]uint64{
		"Version":                 aravis.GVBS_VERSION_REGISTER,
		"Device Mode":             aravis.GVBS_DEVICE_MODE_REGISTER,
		"Device MAC Address High": aravis.GVBS_DEVICE_MAC_HIGH_REGISTER,
		"Device MAC Address Low":  aravis.GVBS_DEVICE_MAC_LOW_REGISTER,
		"Device IP Address":       aravis.GVBS_DEVICE_IP_REGISTER,
		"Device Subnet Mask":      aravis.GVBS_DEVICE_SUBNET_REGISTER,
		"Device Gateway":          aravis.GVBS_DEVICE_GATEWAY_REGISTER,
	}

	fmt.Println("Reading common GigE Vision bootstrap registers:")

	for name, addr := range commonRegisters {
		value, err := device.ReadRegister(addr)
		if err != nil {
			fmt.Printf("  %s (0x%04X): Error - %v\n", name, addr, err)
		} else {
			fmt.Printf("  %s (0x%04X): 0x%08X (%d)\n", name, addr, value, value)
		}
	}

	fmt.Println("\n=== Memory Access Examples ===")

	// Example 2: Read a small chunk of memory from bootstrap area
	fmt.Println("Reading 64 bytes from bootstrap memory area (0x0000-0x003F):")

	memData, err := device.ReadMemory(0x0000, 64)
	if err != nil {
		fmt.Printf("Memory read error: %v\n", err)
	} else {
		fmt.Println("Memory contents (hex dump):")

		for index := 0; index < len(memData); index += 16 {
			end := index + 16
			if end > len(memData) {
				end = len(memData)
			}

			// Print address
			fmt.Printf("  0x%04X: ", index)

			// Print hex values
			for j := index; j < end; j++ {
				fmt.Printf("%02X ", memData[j])
			}

			// Pad with spaces if necessary
			for j := end; j < index+16; j++ {
				fmt.Print("   ")
			}

			// Print ASCII representation
			fmt.Print(" |")

			for j := index; j < end; j++ {
				if memData[j] >= 32 && memData[j] <= 126 {
					fmt.Printf("%c", memData[j])
				} else {
					fmt.Print(".")
				}
			}

			fmt.Println("|")
		}
	}

	fmt.Println("\n=== Feature-based Access (Safer Alternative) ===")

	// Example 3: Show safer feature-based access as alternative
	fmt.Println("Demonstrating safer feature-based access:")

	// Try to get some common features
	commonFeatures := []string{
		"DeviceVendorName",
		"DeviceModelName",
		"DeviceSerialNumber",
		"DeviceVersion",
		"DeviceTemperature",
		"DeviceLinkSpeed",
		"Width",
		"Height",
		"PixelFormat",
		"ExposureTime",
		"Gain",
	}

	for _, feature := range commonFeatures {
		// Try string feature first
		if value, err := device.GetStringFeatureValue(feature); err == nil {
			fmt.Printf("  %s (string): %s\n", feature, value)
			continue
		}

		// Try integer feature
		if value, err := device.GetIntegerFeatureValue(feature); err == nil {
			fmt.Printf("  %s (int): %d\n", feature, value)
			continue
		}

		// Try float feature
		if value, err := device.GetFloatFeatureValue(feature); err == nil {
			fmt.Printf("  %s (float): %.2f\n", feature, value)
			continue
		}

		// Feature not available or not readable
		fmt.Printf("  %s: Not available or not readable\n", feature)
	}

	fmt.Println("\n=== Safety Notes ===")
	fmt.Println("• Register access is primarily for GigE Vision cameras")
	fmt.Println("• Always use feature-based access when possible (safer)")
	fmt.Println("• Direct register writes can damage cameras - use with extreme caution")
	fmt.Println("• Consult camera documentation for specific register maps")
	fmt.Println("• Some registers may be read-only or have side effects")
	fmt.Println("• Test thoroughly in development environments first")

	fmt.Println("\nRegister access example complete!")
}
