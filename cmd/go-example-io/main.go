package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/pkg/errors"
)

var (
	ioModeFlag = flag.String("mode", "", "Mode (read, write, combined) to use when running")
	pathFlag   = flag.String("path", "", "Path of the file to read/write from")
)

// This config should run for 44 total seconds before finishing
const (
	IO_SEGMENT        = 32768
	IO_PERIOD         = 500 * time.Millisecond
	IO_TOTAL          = 1048576
	IO_COMBINED_PAUSE = 4 * time.Second
)

func main() {
	flag.Parse()

	if *ioModeFlag == "" {
		log.Fatal("--mode is required")
	}
	if *pathFlag == "" {
		log.Fatal("--path is required")
	}

	ctx := context.Background()

	// Trap SIGINT cancel the context
	ctx, cancel := context.WithCancel(ctx)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	defer func() {
		signal.Stop(c)
		cancel()
	}()
	go func() {
		select {
		case <-c:
			cancel()
		case <-ctx.Done():
		}
	}()

	err := run(ctx, *ioModeFlag, *pathFlag)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	os.Exit(0)
}

// run runs the main CLI program using the given arguments
// and cancellation context.
func run(ctx context.Context, mode string, path string) error {
	switch mode {
	case "read":
		return runRead(ctx, path)
	case "write":
		return runWrite(ctx, path)
	case "combined":
		return runCombined(ctx, path)
	default:
		return fmt.Errorf("unknown --mode argument %q", mode)
	}
}

// runCombined runs both a write and then a read to the given path,
// sleeping for (IO_COMBINED_PAUSE) in before, after, and in between.
func runCombined(ctx context.Context, path string) error {
	log.Println("Starting combined read/write")
	select {
	case <-ctx.Done():
		return errors.Wrap(ctx.Err(), "sleeping before write cancelled")
	case <-time.After(IO_COMBINED_PAUSE):
	}

	err := runWrite(ctx, path)
	if err != nil {
		return errors.Wrap(err, "error while running write")
	}

	select {
	case <-ctx.Done():
		return errors.Wrap(ctx.Err(), "sleeping between read and write cancelled")
	case <-time.After(IO_COMBINED_PAUSE):
	}

	err = runRead(ctx, path)
	if err != nil {
		return errors.Wrap(err, "error while running read")
	}

	select {
	case <-ctx.Done():
		return errors.Wrap(ctx.Err(), "sleeping after read cancelled")
	case <-time.After(IO_COMBINED_PAUSE):
	}

	log.Println("Finished combined read/write")
	return nil
}

// runRead reads from the given path and computes the SHA-256 digest,
// printing it out to stdout.
// Reads (IO_SEGMENT) bytes every (IO_PERIOD), sleeping in between,
// until the entire file has been read in.
func runRead(ctx context.Context, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return errors.Wrap(err, "failed to open file for reading")
	}
	defer file.Close()

	hasher := sha256.New()
	ticker := time.NewTicker(IO_PERIOD)
	buf := make([]byte, 0, IO_SEGMENT)
	totalRead := 0
	log.Printf("Starting read from %q", path)
	for {
		// Wait for the interval
		select {
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "reading cancelled")
		case <-ticker.C:
		}

		bytesRead, err := file.Read(buf[:cap(buf)])
		if err != nil && err != io.EOF {
			return errors.Wrap(err, "failed to read segment from file")
		}
		totalRead += bytesRead

		// Add the read bytes to the hash
		hasher.Write(buf[:bytesRead])

		if err == io.EOF {
			break
		}
	}

	log.Printf("Reading finished from %q (%d bytes)", path, totalRead)

	// Print the hash out as hex
	computedHash := hex.EncodeToString(hasher.Sum(nil))
	log.Printf("SHA-256 hash: %s", computedHash)
	return nil
}

// runWrite writes to the given path with a sequence of cryptographically random bytes.
// Writes (IO_SEGMENT) bytes every (IO_PERIOD), sleeping in between,
// until (IO_TOTAL) total bytes have been written.
func runWrite(ctx context.Context, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return errors.Wrap(err, "failed to create file for writing")
	}
	defer file.Close()

	ticker := time.NewTicker(IO_PERIOD)
	buf := make([]byte, 0, IO_SEGMENT)
	totalWritten := 0
	numSegments := IO_TOTAL / IO_SEGMENT
	log.Printf("Starting write to %q", path)
	for i := 0; i < numSegments; i++ {
		// Wait for the interval
		select {
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "writing cancelled")
		case <-ticker.C:
		}

		err := generateRandomBytes(buf[:cap(buf)])
		if err != nil {
			return errors.Wrap(err, "failed to generate random bytes to write to file")
		}

		// Append the random bytes to the file
		bytesWritten, err := file.Write(buf[:cap(buf)])
		if err != nil {
			return errors.Wrap(err, "failed to write segment to file")
		}
		totalWritten += bytesWritten
	}

	log.Printf("Writing finished to %q (%d bytes)", path, totalWritten)
	return nil
}

// generateRandomBytes fills len(buffer) bytes in the given buffer
// with cryptographically random bytes.
func generateRandomBytes(buffer []byte) error {
	_, err := rand.Read(buffer)
	if err != nil {
		return errors.Wrap(err, "rand.Read() returned error")
	}

	return nil
}
