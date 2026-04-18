package iprfc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	pbr "github.com/RTradeLtd/grpc/lens/request"

	ipfsapi "github.com/RTradeLtd/go-ipfs-api"
	"github.com/RTradeLtd/iprfc/lens"
)

var (
	// error is returned when we've downloaded the last rfc
	errMoreRFCs = errors.New("no more rfcs to download")
	baseURL     = "https://www.rfc-editor.org/rfc/pdfrfc/"
	// https://www.rfc-editor.org/rfc/pdfrfc/rfc5245.txt.pdf

	// httpClient is a shared HTTP client with a reasonable timeout.
	httpClient = &http.Client{
		Timeout: 60 * time.Second,
	}

	// userAgent mimics a real browser to avoid WAF/bot-protection blocks.
	userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) " +
		"AppleWebKit/537.36 (KHTML, like Gecko) " +
		"Chrome/125.0.0.0 Safari/537.36"
)

// GetRFC gets an RFC number
func GetRFC(num int) string {
	return fmt.Sprintf("rfc%v", num)
}

// FormatURLs returns a list of candidate URLs to try for a given RFC.
// IETF changed their naming convention around RFC 8700.
func FormatURLs(rfc string) []string {
	return []string{
		// Modern convention (direct PDF)
		"https://www.rfc-editor.org/rfc/" + rfc + ".pdf",
		// Legacy convention (txt-based PDF)
		"https://www.rfc-editor.org/rfc/pdfrfc/" + rfc + ".txt.pdf",
	}
}

// GetAndSave downloads an RFC as a PDF and saves it to disk.
func GetAndSave(rfc string) error {
	urls := FormatURLs(rfc)
	var lastErr error

	for _, url := range urls {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			lastErr = fmt.Errorf("creating request for %s: %w", url, err)
			continue
		}
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Accept", "application/pdf")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")

		resp, err := httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("downloading %s: %w", url, err)
			continue
		}
		defer resp.Body.Close()

		// If 404, try the next candidate URL.
		if resp.StatusCode == http.StatusNotFound {
			lastErr = errMoreRFCs
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("unexpected status %d for %s", resp.StatusCode, url)
			continue
		}

		// Validate Content-Type to ensure it's a PDF.
		ct := resp.Header.Get("Content-Type")
		if !strings.Contains(ct, "application/pdf") {
			lastErr = fmt.Errorf("expected application/pdf, got %q for %s", ct, url)
			continue
		}

		// If we got here, we have a valid PDF. Save it.
		outFile, err := os.Create(rfc + ".pdf")
		if err != nil {
			return fmt.Errorf("creating file %s.pdf: %w", rfc, err)
		}
		defer outFile.Close()

		if _, err := io.Copy(outFile, resp.Body); err != nil {
			os.Remove(rfc + ".pdf")
			return fmt.Errorf("writing %s.pdf: %w", rfc, err)
		}

		return nil // Success!
	}

	return lastErr
}

// maxConsecutiveMisses is the number of consecutive 404s before auto-stopping
// when running in unlimited mode (max == 0).
const maxConsecutiveMisses = 100

// DownloadOptions configures the behavior of DownloadAndSave.
type DownloadOptions struct {
	Min       int  // first RFC number to download (default: 1)
	Max       int  // last RFC number to download, 0 means unlimited
	Overwrite bool // if false, skip files that already exist on disk
}

// DownloadAndSave downloads RFCs in the range [opts.Min, opts.Max] and saves
// them as PDF files in the current directory.
//
// When opts.Max is 0 (unlimited mode), downloading stops automatically after
// 100 consecutive 404 responses, indicating the end of the RFC numbering space.
//
// Progress is printed to stdout for every RFC processed.
func DownloadAndSave(opts DownloadOptions) {
	if opts.Min < 1 {
		opts.Min = 1
	}

	var (
		downloaded      int
		skipped         int
		missed          int
		consecutiveMiss int
		startTime       = time.Now()
	)

	for count := opts.Min; ; count++ {
		// Stop if we've reached the upper bound (when max > 0).
		if opts.Max != 0 && count > opts.Max {
			break
		}

		rfc := GetRFC(count)
		filename := rfc + ".pdf"

		// Skip already downloaded files unless --overwrite is set.
		if !opts.Overwrite {
			if _, err := os.Stat(filename); err == nil {
				skipped++
				fmt.Printf("[SKIP]  %s (already exists)\n", filename)
				consecutiveMiss = 0 // existing file resets the miss counter
				continue
			}
		}

		err := GetAndSave(rfc)
		switch err {
		case nil:
			downloaded++
			consecutiveMiss = 0
			fmt.Printf("[OK]    %s  (downloaded: %d, skipped: %d, missed: %d)\n",
				filename, downloaded, skipped, missed)
		case errMoreRFCs:
			missed++
			consecutiveMiss++
			fmt.Printf("[MISS]  %s  (not found on server)\n", filename)
			// In unlimited mode, stop after too many consecutive misses.
			if opts.Max == 0 && consecutiveMiss >= maxConsecutiveMisses {
				fmt.Printf("\n--- Auto-stop: %d consecutive misses reached. "+
					"Assuming end of RFC numbering. ---\n", maxConsecutiveMisses)
				break
			}
			continue
		default:
			log.Fatalf("error downloading rfc: %s", err)
		}

		// Break out of the outer for-loop if we hit the auto-stop above.
		if opts.Max == 0 && consecutiveMiss >= maxConsecutiveMisses {
			break
		}
	}

	elapsed := time.Since(startTime).Round(time.Second)
	fmt.Printf("\n=== Done in %s. Downloaded: %d, Skipped: %d, Missed: %d ===\n",
		elapsed, downloaded, skipped, missed)
}

// StoreAndIndex is used to store a file on IPFS and index it
//
// It reads all files in the current directory, adds it to IPFS, and then indexing it against Lens
func StoreAndIndex(ctx context.Context, sh *ipfsapi.Shell, lc *lens.Client, index bool) error {
	files, err := os.ReadDir(".")
	if err != nil {
		return err
	}
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".pdf") {
			fh, err := os.Open(file.Name())
			if err != nil {
				return err
			}
			hash, err := sh.Add(fh)
			if err != nil {
				return err
			}
			fmt.Printf("added\t%s\t%s\n", hash, file.Name())
			if index {
				if err := Index(ctx, lc, hash); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// Index is used to index a hash against lens
func Index(ctx context.Context, lc *lens.Client, hash string) error {
	_, err := lc.Index(ctx, &pbr.Index{
		Type:       "ipld",
		Identifier: hash,
	})
	return err
}
