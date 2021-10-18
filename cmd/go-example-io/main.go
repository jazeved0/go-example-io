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
	ioModeFlag     = flag.String("mode", "", "Mode (read, write) to use when running")
	pathFlag       = flag.String("path", "", "Path of the file to read/write from")
	iterSleepFlag  = flag.Duration("iter-sleep", 1*time.Millisecond, "Amount of time to sleep between read/write iterations (a single block read/written)")
	blockCountFlag = flag.Int("blocks", 2048, "Number of blocks to read/write")
	blockSizeFlag  = flag.Int("block-size", 32768, "The size of each block to read/write")
	syncWriteFlag  = flag.Bool("sync", false, "Whether to sync at the end of a write operation")
)

type Command struct {
	mode       string
	path       string
	iterSleep  time.Duration
	blockCount int
	blockSize  int
	syncWrite  bool
}

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

	command := Command{
		mode:       *ioModeFlag,
		path:       *pathFlag,
		iterSleep:  *iterSleepFlag,
		blockCount: *blockCountFlag,
		blockSize:  *blockSizeFlag,
		syncWrite:  *syncWriteFlag,
	}

	err := command.run(ctx)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	os.Exit(0)
}

// run runs the main CLI program using the given arguments
// and cancellation context.
func (c *Command) run(ctx context.Context) error {
	switch c.mode {
	case "read":
		return c.runRead(ctx)
	case "write":
		return c.runWrite(ctx)
	default:
		return fmt.Errorf("unknown --mode argument %q", c.mode)
	}
}

// runRead reads from the given path and computes the SHA-256 digest,
// printing it out to stdout.
// Reads (c.blockSize) bytes every (c.iterSleep), sleeping in between,
// until the entire file has been read in.
func (c *Command) runRead(ctx context.Context) error {
	file, err := os.Open(c.path)
	if err != nil {
		return errors.Wrap(err, "failed to open file for reading")
	}
	defer file.Close()

	hasher := sha256.New()
	ticker := time.NewTicker(c.iterSleep)
	buf := make([]byte, 0, c.blockSize)
	totalRead := 0
	log.Printf("Starting read from %q", c.path)
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

	log.Printf("Reading finished from %q (%d bytes)", c.path, totalRead)

	// Print the hash out as hex
	computedHash := hex.EncodeToString(hasher.Sum(nil))
	log.Printf("SHA-256 hash: %s", computedHash)
	return nil
}

// runWrite writes to the given path with a sequence of cryptographically random bytes.
// Writes (c.blockSize) bytes every (c.iterSleep), sleeping in between,
// until (c.blockCount) total bytes have been written.
// If (c.syncWrite), runWrite also calls file.Sync() at the end to persist changes.
func (c *Command) runWrite(ctx context.Context) error {
	file, err := os.Create(c.path)
	if err != nil {
		return errors.Wrap(err, "failed to create file for writing")
	}
	defer file.Close()

	ticker := time.NewTicker(c.iterSleep)
	buf := make([]byte, 0, c.blockSize)
	totalWritten := 0
	log.Printf("Starting write to %q", c.path)
	for i := 0; i < c.blockCount; i++ {
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

	// Sync the file if that behavior is enabled
	if c.syncWrite {
		err = file.Sync()
		if err != nil {
			return errors.Wrap(err, "failed to sync the written file")
		}
	}

	log.Printf("Writing finished to %q (%d bytes)", c.path, totalWritten)
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
