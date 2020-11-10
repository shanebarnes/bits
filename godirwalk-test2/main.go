package main

import (
	"flag"
	"fmt"
	"github.com/karrick/godirwalk"
	"github.com/dustin/go-humanize"
	"os"
	"runtime"
	"strconv"
	"sync"
	"time"
)

var (
	_byteCount int64
	_fileCount int64
	_mtx sync.Mutex
)

func main() {
	dir := flag.String("dir", "", "directory to walk")
	reportInt := flag.Int("report", 15, "report interval in seconds")
	noStat := flag.Bool("nostat", false, "disable full stat on each file discovered")
	workers := flag.Int("threads", runtime.NumCPU(), "number of threads")
	flag.Parse()

	if *reportInt < 1 {
		panic("Report interval must be greater than zero")
	} else if *workers < 1 {
		panic("Threads must be greater than zero")
	} else if *workers > 256 {
		panic("Threads must be less than or equal to 256")
	}
	reportDur := time.Duration(*reportInt) * time.Second

	fmt.Println("Starting walk of", *dir, "with", strconv.FormatInt(int64(*workers), 10), "threads and full stat mode =", !(*noStat))
	t0 := time.Now()
	tr := t0

	var wg sync.WaitGroup
	wg.Add(*workers)
	ch := make(chan string, *workers)
	for i := 0; i < *workers; i++ {
		go stat((!*noStat), &ch, &wg)
	}

	err := godirwalk.Walk(*dir, &godirwalk.Options{
		Callback: func(osPathname string, de *godirwalk.Dirent) error {
			ch <- osPathname
			lastReport := time.Since(tr)
			if lastReport > reportDur {
				tr = tr.Add(reportDur)
				_mtx.Lock()
				fileCount := _fileCount
				_mtx.Unlock()
				fmt.Println("Found", fileCount, "files in", time.Since(t0))
			}
			return nil
		},
		Unsorted: true, // (optional) set true for faster yet non-deterministic enumeration (see godoc)
	})

	close(ch)
	wg.Wait()
	t1 := time.Now()
	fmt.Println("Found a total of", _fileCount, "files,", humanize.Bytes(uint64(_byteCount)), "in", t1.Sub(t0))
	fmt.Println("Completed walk of", *dir, ", err: ", err)
}

func stat(fullStat bool, ch *chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	for path := range *ch {
		fileValid := true
		fileSize := int64(0)
		if fullStat {
			if fi, err := os.Lstat(path); err == nil {
				if !fi.IsDir() {
					fileSize = fi.Size()
				}
			} else {
				fileValid = false
			}
		}

		if !fullStat || fileValid {
			_mtx.Lock()
			_byteCount += fileSize
			_fileCount += 1
			_mtx.Unlock()
		}
	}
}
