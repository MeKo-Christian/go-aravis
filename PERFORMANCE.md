# Performance Optimization Guide

This document describes the lower-overhead paths available in go-aravis and when they are
worth using.

> **On numbers.** This guide deliberately contains no throughput, latency, or CPU figures.
> Earlier revisions published a table of them; they were not produced by any benchmark in
> this repository and could not be reproduced from it. The benchmarks under `tests/` skip
> without camera hardware, so nothing here can be measured in CI except allocation counts.
> The one quantitative claim that survives is the allocation behavior of `GetDataInto`,
> which `TestGetDataIntoZeroAllocations` asserts with `testing.AllocsPerRun`. Measure your
> own camera and network before sizing a system.

## Overview

Three mechanisms reduce per-call overhead, plus the error type that reports failures:

1. **String Constant Caching** - Avoids repeated C string allocations for feature names
2. **Zero-Copy Buffer Access** - Avoids memory copies for image data
3. **Fast API Methods** - Variants of common operations built on the interned strings

Error handling is *not* an optimization. It used to be described as one; see
[Error Handling](#error-handling) for what it actually does.

## String Constant Caching

### Problem

Camera parameter accesses that go through a named GenICam feature allocate and free a C
string for that name on every call. `GetWidth()` and `GetHeight()` do this. Note that not
every setter does — `SetExposureTime()`, for instance, calls a dedicated Aravis entry
point and never builds a feature-name string:

```go
// Inefficient - creates C.CString("Width") every call
width, err := camera.GetWidth()
```

### Solution

Use the new `*Fast()` methods that use pre-cached C strings:

```go
// Efficient - uses cached C string
width, err := camera.GetWidthFast()
height, err := camera.GetHeightFast()
exposure, err := camera.GetExposureTimeFast()
gain, err := camera.GetGainFast()
```

### What this buys you

- **No per-call C string allocation** for the common GenICam features interned at
  startup. A feature name outside that set falls back to a temporary C string
  per call, so the cache cannot grow without bound when names are generated at
  runtime or come from user input
- Most useful in streaming loops that adjust parameters every frame

The saving is one small allocation and free per call. On real hardware the device
round-trip dominates by orders of magnitude, so do not expect this to move a frame rate —
it removes GC pressure, not wire time.

## Zero-Copy Buffer Access

### Problem

Standard `GetData()` copies the entire frame buffer:

```go
// Inefficient - copies entire frame buffer
data, err := buffer.GetData() // Allocates new []byte every time
```

### Solutions

#### 1. Zero-Copy Slice Access (Fastest, but requires care)

```go
// Efficient - direct access to C memory
dataSlice, err := buffer.GetDataSlice()
if err != nil {
    return err
}

// WARNING: dataSlice aliases memory owned by Aravis. It is invalidated as soon as
// the buffer goes back to the stream via stream.PushBuffer — process the data
// before that call, or copy what you need to keep.
```

#### 2. Pre-allocated Buffer Copy (Fast and safe)

The copy is yours and outlives `PushBuffer`, unlike the zero-copy slice. It is only
stable until the next frame is copied into the same destination, though, so anything
retaining a frame past the current iteration needs its own copy or its own destination
buffer.

```go
// Pre-allocate buffer once
dataBuffer := make([]byte, payloadSize)

// In streaming loop - no allocations
for {
    buffer, err := stream.TimeoutPopBuffer(timeout)
    // Push back whatever we got: a popped buffer is ours, and a non-nil buffer
    // can arrive together with a non-nil error.
    if buffer.IsNil() {
        continue
    }
    if err != nil {
        stream.PushBuffer(buffer)
        continue
    }

    // Copy into pre-allocated buffer
    bytesRead, err := buffer.GetDataInto(dataBuffer)
    if err == nil {
        process(dataBuffer[:bytesRead])
    }

    stream.PushBuffer(buffer)
}
```

#### 3. Unsafe Pointer Access (Expert use only)

```go
// Direct pointer access for C interop
ptr, size, err := buffer.GetDataUnsafe()
if err != nil {
    return err
}
// Use unsafe.Pointer for direct memory access
```

### What this buys you

- `GetDataSlice` and `GetDataUnsafe` copy nothing at all; they hand you the C buffer
- `GetDataInto` allocates nothing, verified by `TestGetDataIntoZeroAllocations`. Reaching
  a true zero also required keeping Go pointers out of the cgo call: passing a Go pointer
  to `arv_buffer_get_data` makes cgo heap-allocate the local, so a C wrapper returns
  data and size together in a struct instead.
- `GetData` allocates a fresh `[]byte` per frame. At megabytes per frame and tens of
  frames per second, that is the single largest source of GC pressure in a naive loop.

## Error Handling

This is a correctness feature, not a performance one. Earlier revisions of this document
claimed error pooling reduced allocation overhead; the pool was a no-op — it never
returned anything to itself — and has been removed, along with the shared mutable error
values it handed out.

### Features

- **Stable messages for known error codes** - Common Aravis device errors use a
  fixed message table instead of converting the GLib string. This avoids one
  `C.GoString` per error; it does not avoid allocating the error.
- **Error code access** - Structured error information
- **Caller-owned errors** - Every error is freshly allocated, so callers can
  never mutate shared state. This is deliberate, and costs one allocation per error.

### Usage

```go
// Errors carry an error code for programmatic handling
if err != nil {
    var aravisErr *aravis.AravisError
    if errors.As(err, &aravisErr) {
        switch aravisErr.Code {
        case aravis.DEVICE_ERROR_TIMEOUT:
            // Handle timeout specifically
        case aravis.DEVICE_ERROR_NOT_FOUND:
            // Handle device not found
        default:
            // Handle other errors
        }
    }
}
```

## High-Performance Streaming Pattern

A streaming loop whose own frame handling allocates nothing. Pick **one** of the two
data-access options — the zero-copy read and the pre-allocated copy are alternatives,
not steps.

Only the successful path through the wrapper is allocation-free, and only
`GetDataInto` is asserted to be so by a test. A timed-out iteration allocates a fresh
error, and whatever `process` does is its own business.

```go
func stream(deviceId string, timeout time.Duration, process func([]byte)) error {
    camera, err := aravis.NewCamera(deviceId)
    if err != nil {
        return err
    }
    defer camera.Close()

    // Use fast methods for configuration
    camera.SetExposureTimeFast(10000) // 10 ms, in the camera's exposure unit
    camera.SetGainFast(1.0)

    payloadSize, err := camera.GetPayloadSize()
    if err != nil {
        return err
    }

    stream, err := camera.CreateStream()
    if err != nil {
        return err
    }
    defer stream.Close()

    // Fill the acquisition queue before starting
    const numBuffers = 5
    for range numBuffers {
        buffer, err := aravis.NewBuffer(payloadSize)
        if err != nil {
            return err
        }
        stream.PushBuffer(buffer)
    }

    // Pre-allocate the destination once, reuse it every frame
    dataBuffer := make([]byte, payloadSize)

    if err := camera.StartAcquisition(); err != nil {
        return err
    }
    defer camera.StopAcquisition()

    for {
        buffer, err := stream.TimeoutPopBuffer(timeout)

        // A popped buffer belongs to us, and the pop can hand back a valid buffer
        // together with a non-nil error. Branch on IsNil, not on err, or the queue
        // drains one buffer per error until acquisition stalls for good.
        if buffer.IsNil() {
            continue // Nothing to recycle
        }

        if err == nil {
            if status, serr := buffer.GetStatus(); serr == nil && status == aravis.BUFFER_STATUS_SUCCESS {
                // Option 1 — zero-copy. Fastest, but the slice is invalidated by
                // the PushBuffer below, so process must not retain it.
                //
                //  if dataSlice, derr := buffer.GetDataSlice(); derr == nil {
                //      process(dataSlice)
                //  }

                // Option 2 — pre-allocated copy. Survives PushBuffer, but every
                // iteration overwrites dataBuffer, so process must copy anything
                // it keeps beyond the current frame.
                if bytesRead, derr := buffer.GetDataInto(dataBuffer); derr == nil {
                    process(dataBuffer[:bytesRead])
                }
            }
        }

        stream.PushBuffer(buffer)
    }
}
```

## Best Practices

### DO:

- Use `*Fast()` methods for frequently accessed parameters
- Pre-allocate buffers and reuse them in streaming loops
- Use `GetDataSlice()` for zero-copy when you can process data immediately
- Use `GetDataInto()` with pre-allocated buffers for safe zero-allocation access
- Set appropriate buffer counts (3-10) based on your processing speed

### DON'T:

- Call `GetData()` in tight loops (allocates every time)
- Keep references to `GetDataSlice()` results after returning buffers
- Use regular methods for high-frequency parameter access
- Allocate new buffers in streaming loops

## Thread Safety

All performance optimizations maintain the same thread safety characteristics as the original methods:

- The string cache is filled once at startup and never written again, so reads
  need no locking
- Buffer operations require external synchronization as before
- Error handling optimizations are thread-safe
