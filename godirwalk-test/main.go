package main

import (
	"flag"
	"fmt"
	"github.com/karrick/godirwalk"
	"io/ioutil"
	"os"
	"runtime"
	"sync"
	"time"
)

var (
	MAX_FILES int
	MAX_SETS int
	MAX_THREADS int
)

func handleErr(fn func() error) {
	err := fn()
	if err != nil {
		panic(err)
	}
}

func init() {
	files := flag.Int("files", 100, "file count")
	sets := flag.Int("sets", 3, "set count")
	threads := flag.Int("threads", runtime.GOMAXPROCS(0), "thread count")

	flag.Parse()

	MAX_SETS = *sets
	MAX_FILES = *files
	MAX_THREADS = *threads
}

func main() {
	var dirname string

	handleErr(func() error {
		var err error
		dirname, err = ioutil.TempDir(os.TempDir(), "walker*.noindex")
		fmt.Println("Created directory:", dirname)
		return err
	})

	//var files []string
	var files sync.Map
	for i := 0; i < MAX_SETS; i++ {
		//fmt.Println("Creating", MAX_FILES, "files in set", i+1)
		//sub := fmt.Sprintf("set%d", i+1)
		//os.Mkdir(dirname + string(os.PathSeparator) + sub, 0777)
		//files.Store(dirname + string(os.PathSeparator) + sub, 0)
		for j := 0; j < MAX_FILES; j++ {
			handleErr(func() error {
				name := fmt.Sprintf("%s%s%dk.set%d.file%d", dirname, string(os.PathSeparator), MAX_SETS*MAX_FILES/1000, i+1, j+1)
				//tmpfile, err := os.Create(name)
				files.Store(name, 0)
				//files.Store(tmpfile.Name(), 0)
				//files = append(files, tmpfile.Name())
				//fmt.Println("Creating file:", tmpfile.Name())
				//tmpfile.Close()
				//return err
				return nil
			})
		}
	}

	defer handleErr(func() error {
		err := os.RemoveAll(dirname)
		fmt.Println("Removed directory:", dirname)
		return err
	})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		files.Range(func (key, value interface{}) bool {
			handleErr(func() error {
				tmpfile, err := os.Create(key.(string))
				defer tmpfile.Close()
				return err
			})
			return true
		})
		fmt.Println("Created", MAX_SETS*MAX_FILES, "files")
		time.Sleep(10*time.Second)
		//for len(files) > 0 {
		files.Range(func(key, value interface{}) bool {
			//i := rand.Intn(len(files)) * 0
			//i := len(files)-1
			//i := len(files)/2
			//fmt.Printf("Removing file #%d: %s\n", i+1, files[i])
			handleErr(func() error {
				//return os.Remove(files[i])
				return os.Remove(key.(string))
			})
			//files = append(files[:i], files[i+1:]...)
			files.Delete(key.(string))
			//time.Sleep(1*time.Millisecond)
			return true
		})
		wg.Done()
	} ()

	ch := make(chan string, MAX_THREADS)
	for i := 0; i < MAX_THREADS; i++ {
		var ok bool = true
		var filename string
		wg.Add(1)
		go func() {
			for ok {
				select {
					case filename, ok = <- ch:
						if ok {
							// Do something
							//os.Remove(filename)
							//if _, exists := files.Load(filename); !exists {
							//	fmt.Println("Detected new file", filename)
							//}
							//if _, err := os.Lstat(filename); err != nil {
							//	fmt.Println("Failed to stat file", filename, ":", err)
							//}
							//time.Sleep(10*time.Microsecond)
						}
				}
			}
			wg.Done()
		}()
	}

	handleErr(func() error {
		var err error
		counter := int64(1)

		//rmCount := 0
		time.Sleep(time.Second)
		for err == nil && counter > 0 {
			errCbCount := 0
			//errStatCount := 0
			counter = 0
			fmt.Println("Walking directory:", dirname)
			var dup sync.Map
			//err := filepath.Walk(dirname, func(osPathname string, info os.FileInfo, err error) error {
			err = godirwalk.Walk(dirname, &godirwalk.Options{
				Callback: func(osPathname string, de *godirwalk.Dirent) error {
					if _, exists := dup.Load(osPathname); exists {
						//fmt.Println("Duplicate entry in same walk cycle:", osPathname)
					} else {
						dup.Store(osPathname, 0)
					}
					if osPathname != dirname {
						counter++
						ch <- osPathname
					}

					return nil
				},
				ErrorCallback: func(str string, err error) godirwalk.ErrorAction {
					//fmt.Println("!!! Walker ErrorCallback:", str, " ", err)
					errCbCount++
					return godirwalk.SkipNode
				},
				Unsorted: true,
			})

			if err != nil {
				fmt.Println("!!!! Encountered error walking directory:", err)
			}
			fmt.Println("Found", counter, "files in directory", ", error callbacks: ", errCbCount)
			if counter > 0 {
				time.Sleep(10 * time.Millisecond)
			}
		}
		close(ch)
		return err
	})

	wg.Wait()
}
