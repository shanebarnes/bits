package main

import (
	"flag"
	"fmt"
	"github.com/karrick/godirwalk"
	"time"
)

func main() {
	dir := flag.String("dir", "", "directory to walk")
	flag.Parse()
	fmt.Println("Starting walk of", *dir)
	cbCount := 0
	t0 := time.Now()
	godirwalk.Walk(*dir, &godirwalk.Options{
		Callback: func(osPathname string, de *godirwalk.Dirent) error {
			cbCount++
			return nil
		},
		Unsorted: true, // (optional) set true for faster yet non-deterministic enumeration (see godoc)
	})
	t1 := time.Now()
	fmt.Println("Found", cbCount, "files in", t1.Sub(t0))
	fmt.Println("Completed walk of", *dir)
}
