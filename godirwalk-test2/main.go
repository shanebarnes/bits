package main

import (
	"flag"
	"fmt"
	"github.com/karrick/godirwalk"
	"time"
)

func main() {
	dir := flag.String("dir", "", "directory to walk")
	reportInt := flag.Int("report", 15, "report interval in seconds")
	flag.Parse()

	if *reportInt < 1 {
		panic("Report interval must be greater than zero")
	}
	reportDur := time.Duration(*reportInt) * time.Second

	fmt.Println("Starting walk of", *dir)
	cbCount := 0
	t0 := time.Now()
	tr := t0

	godirwalk.Walk(*dir, &godirwalk.Options{
		Callback: func(osPathname string, de *godirwalk.Dirent) error {
			cbCount++
			lastReport := time.Since(tr)
			if lastReport > reportDur {
				tr = tr.Add(reportDur)
				fmt.Println("Found", cbCount, "files in", time.Since(t0))
			}
			return nil
		},
		Unsorted: true, // (optional) set true for faster yet non-deterministic enumeration (see godoc)
	})

	t1 := time.Now()
	fmt.Println("Found a total of", cbCount, "files in", t1.Sub(t0))
	fmt.Println("Completed walk of", *dir)
}
