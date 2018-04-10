package main

import (
	//"fmt"
	"log"
	"os"
	//"time"
	"path"
	"regexp"
	"runtime"
	"sort"
	"sync"
	"time"

	pcre "github.com/gijsbers/go-pcre"
	gtk "github.com/gotk3/gotk3/gtk"

	"testing"
)

func TestBuckets(t *testing.T) {
	log.Println("running TestBuckets")

	mem := ResultMemory{
		NewNameBucket(),
		NewModTimeBucket(),
		NewSizeBucket(),
	}
	finish := make(chan struct{})

	//directories := []string{os.Getenv("HOME")}
	directories := []string{os.Getenv("HOME"), "/usr", "/var", "/sys", "/opt", "/etc", "/bin", "/sbin"}
	if testing.Short() {
		directories = []string{os.Getenv("HOME")}
	}

	newdirs := make(chan string)
	cores := runtime.NumCPU()

	var wg sync.WaitGroup
	log.Println("starting Crawl on", cores, "cores")
	wg.Add(1)
	go Crawler(&wg, cores*2, mem, newdirs, finish)
	for _, dir := range directories {
		newdirs <- dir
	}
	wg.Wait()
	close(finish)
	log.Println("Crawl terminated")

	searchterm := "golocate"
	query, _ := regexp.Compile(searchterm)
	cache := MatchCache{make(map[string]bool), make(map[string]bool)}
	abort := make(chan struct{})
	taken := make(chan *FileEntry)

	var byname, bymodtime, bysize []*FileEntry

	taker := func(xs *[]*FileEntry) {
		for {
			entry := <-taken
			if entry == nil {
				return
			}
			*xs = append(*xs, entry)
		}
	}

	go taker(&byname)
	mem.byname.Take(&cache, SORT_BY_NAME, gtk.SORT_ASCENDING, query, 1000, abort, taken)

	go taker(&bymodtime)
	mem.bymodtime.Take(&cache, SORT_BY_MODTIME, gtk.SORT_ASCENDING, query, 1000, abort, taken)

	go taker(&bysize)
	mem.bysize.Take(&cache, SORT_BY_SIZE, gtk.SORT_ASCENDING, query, 1000, abort, taken)

	log.Println("len(byname):", len(byname))
	log.Println("len(bymodtime):", len(bymodtime))
	log.Println("len(bysize):", len(bysize))

	//Print(mem.byname.(*NameBucket), 0)
	//Print(mem.bymodtime.(*ModTimeBucket), 0)
	//Print(mem.bysize.(*SizeBucket), 0)

	var lastentry *FileEntry
	WalkEntries(mem.bymodtime.(*Node), gtk.SORT_ASCENDING, func(entry *FileEntry) bool {
		if lastentry == nil {
			lastentry = entry
			return true
		} else {
			if ModTimeThreshold(entry.modtime).Less(ModTimeThreshold(lastentry.modtime)) {
				t.Error("ModTimeBucket Walk could not assert ASCENDING sorting")
				return false
			}
			lastentry = entry
			return true
		}
	})

	lastentry = nil
	WalkEntries(mem.bymodtime.(*Node), gtk.SORT_DESCENDING, func(entry *FileEntry) bool {
		if lastentry == nil {
			lastentry = entry
			return true
		} else {
			if ModTimeThreshold(lastentry.modtime).Less(ModTimeThreshold(entry.modtime)) {
				t.Error("ModTimeBucket Walk could not assert DESCENDING sorting")
				return false
			}
			lastentry = entry
		}
		return true
	})

	WalkNodes(mem.bysize.(*Node), gtk.SORT_ASCENDING, func(child Bucket) bool {
		node := child.Node()

		if !sort.IsSorted(SortedBySize(node.sorted)) {
			t.Error("Found a node.sorted that is not sorted")
			return false
		}

		if len(node.children) == 1 {
			t.Error("Found a node with only one children")
			return false
		}

		for _, entry := range node.sorted {
			if node.threshold != nil {
				if !SizeThreshold(entry.size).Less(node.threshold) {
					t.Error("Found an entry.size that is not less then its threshold")
					return false
				}
			}
		}
		return true
	})

	log.Println("TestBuckets finished")
}

func TestLess(t *testing.T) {
	if NameThreshold("=.html").Less(NameThreshold("9")) {
		t.Error("=.html < 9")
	}

	if NameThreshold("=.html").Less(NameThreshold("1")) {
		t.Error("=.html < 1")
	}

	if NameThreshold("b").Less(NameThreshold("aaaaaaaaaaaaaaaaaaaaaaaaa")) {
		t.Error("b < aaaaaaaaaaaaaaaaaaaaaaaaa")
	}

	if NameThreshold("a").Less(NameThreshold("0")) {
		t.Error("a < 0")
	}

	if NameThreshold("a").Less(NameThreshold("A")) {
		t.Error("a < A")
	}

	if ModTimeThreshold(time.Now().Add(-time.Minute)).Less(ModTimeThreshold(time.Now())) {
		t.Error("time.Now().Add(-time.Minute) < time.Now")
	}

	if SizeThreshold(0).Less(SizeThreshold(1)) {
		t.Error("0 < 1")
	}

	log.Println("TestLess finished")
}

func BenchmarkRegexpBuiltin(b *testing.B) {
	b.StopTimer()

	mem := ResultMemory{
		NewNameBucket(),
		NewModTimeBucket(),
		NewSizeBucket(),
	}
	directories := []string{path.Join(os.Getenv("GOPATH"))}
	newdirs := make(chan string)

	cores := runtime.NumCPU()

	finish := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go Crawler(&wg, cores, mem, newdirs, finish)
	for _, dir := range directories {
		newdirs <- dir
	}
	wg.Wait()
	close(finish)

	searchterm1 := ".*\\.cc$"
	query1, _ := regexp.Compile(searchterm1)
	searchterm2 := ".*\\.cc"
	query2, _ := regexp.Compile(searchterm2)
	searchterm3 := ".*\\."
	query3, _ := regexp.Compile(searchterm3)

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		WalkEntries(mem.bymodtime.(*Node), gtk.SORT_ASCENDING, func(entry *FileEntry) bool {
			query1.MatchString(entry.name)
			query1.MatchString(entry.path)

			query2.MatchString(entry.name)
			query2.MatchString(entry.path)

			query3.MatchString(entry.name)
			query3.MatchString(entry.path)
			return true
		})
	}
}

func BenchmarkRegexpPCRE(b *testing.B) {
	runtime.LockOSThread()

	b.StopTimer()

	mem := ResultMemory{
		NewNameBucket(),
		NewModTimeBucket(),
		NewSizeBucket(),
	}
	directories := []string{path.Join(os.Getenv("GOPATH"))}
	newdirs := make(chan string)

	cores := runtime.NumCPU()

	finish := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go Crawler(&wg, cores, mem, newdirs, finish)
	for _, dir := range directories {
		newdirs <- dir
	}
	wg.Wait()
	close(finish)

	searchterm1 := ".*\\.cc$"
	searchterm2 := ".*\\.cc"
	searchterm3 := ".*\\."

	pcrere1, pcreerr1 := pcre.CompileJIT(searchterm1, pcre.DOTALL|pcre.UTF8|pcre.UCP, pcre.STUDY_JIT_COMPILE)
	if pcreerr1 != nil {
		log.Println(pcreerr1)
	}

	pcrere2, pcreerr2 := pcre.CompileJIT(searchterm2, pcre.DOTALL|pcre.UTF8|pcre.UCP, pcre.STUDY_JIT_COMPILE)
	if pcreerr2 != nil {
		log.Println(pcreerr2)
	}

	pcrere3, pcreerr3 := pcre.CompileJIT(searchterm3, pcre.DOTALL|pcre.UTF8|pcre.UCP, pcre.STUDY_JIT_COMPILE)
	if pcreerr3 != nil {
		log.Println(pcreerr3)
	}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		WalkEntries(mem.bymodtime.(*Node), gtk.SORT_ASCENDING, func(entry *FileEntry) bool {
			namematcher1 := pcrere1.MatcherString(entry.name, 0)
			namematcher1.Matches()
			pathmatcher1 := pcrere1.MatcherString(entry.path, 0)
			pathmatcher1.Matches()

			namematcher2 := pcrere2.MatcherString(entry.name, 0)
			namematcher2.Matches()
			pathmatcher2 := pcrere2.MatcherString(entry.path, 0)
			pathmatcher2.Matches()

			namematcher3 := pcrere3.MatcherString(entry.name, 0)
			namematcher3.Matches()
			pathmatcher3 := pcrere3.MatcherString(entry.path, 0)
			pathmatcher3.Matches()

			return true
		})
	}

	runtime.UnlockOSThread()

}
