package main

import (
	"log"
	"os"
	"os/exec"
	"path"
	"regexp"
	//"strconv"
	"strings"
	"sync"
	//"time"

	"github.com/go-cmd/cmd"
	"gotk3/gtk"
)

type FileEntry struct {
	path     string
	fileinfo os.FileInfo
	urgency  int
}

func DirToDbName(dir string) string {
	name := strings.Replace(strings.Trim(dir, "/"), "/", "-", -1)
	if len(name) > 0 {
		name = "-" + name
	}
	name = "golocate" + name
	return name
}

func RunUpdatedbCommand(dbname, updatedb, prunenames, prunepaths, dir string) {
	updatedbCmd := cmd.NewCmd(updatedb,
		"--require-visibility", "0",
		"--prunenames", prunenames,
		"--prunepaths", prunepaths,
		"-o", "/dev/shm/"+dbname+".db",
		"-U", dir)
	updatedbStatusChan := updatedbCmd.Start()
	<-updatedbStatusChan
}

func RunLocateCommand(dbname, mlocate, dir, query string) []string {
	mlocateCmd := cmd.NewCmd(mlocate, "-d", "/dev/shm/"+dbname+".db", "-r", query)
	mlocateStatusChan := mlocateCmd.Start()
	<-mlocateStatusChan

	status := mlocateCmd.Status()
	return status.Stdout
}

func SplitDirectories(directories []string) ([]string, string) {
	home := os.Getenv("HOME")

	var splittedDirectories []string
	var additionalPrunepaths string
	for _, dir := range directories {
		if matchedHome, err := regexp.MatchString("^"+regexp.QuoteMeta(home), dir); matchedHome {
			if homeHandle, err := os.Open(dir); err != nil {
				log.Fatal("Could not open home:", err)
			} else {
				if homeDirEntries, err := homeHandle.Readdir(0); err != nil {
					log.Fatal("Could not read home:", err)
				} else {
					for _, fileinfo := range homeDirEntries {
						if matchedVisible, err := regexp.MatchString("^[^\\.].*", fileinfo.Name()); err != nil {
							log.Fatal("Error when matching regexp:", err)
						} else if fileinfo.IsDir() && matchedVisible {
							subDir := path.Join(dir, fileinfo.Name())
							splittedDirectories = append(splittedDirectories, subDir)
							additionalPrunepaths += " " + subDir
						}
					}
				}
			}
		} else if err != nil {
			log.Fatal("Error when matching regexp:", err)
		}
	}

	return splittedDirectories, additionalPrunepaths
}

var lastQuery = ""

func SearchDirectory(searchWg *sync.WaitGroup, collect chan []FileEntry, updatedb, prunenames, prunepaths, mlocate, dir, query string) {
	dbname := DirToDbName(dir)
	if len(query) > len(lastQuery) {
		RunUpdatedbCommand(dbname, updatedb, prunenames, prunepaths, dir)
		lastQuery = query
	}
	stdout := RunLocateCommand(dbname, mlocate, dir, query)
	log.Println(dir, ": ", len(stdout))

	entries := make(chan FileEntry)
	var statWg sync.WaitGroup
	for i := 0; i < len(stdout); i++ {
		statWg.Add(1)
		filepath := stdout[i]
		dirpath := path.Dir(filepath)

		go func() {
			fileinfo, err := os.Lstat(filepath)
			if err != nil {
				log.Println("Error reading file:", err)
			} else {
				entries <- FileEntry{path: dirpath, fileinfo: fileinfo, urgency: 0}
			}
			defer statWg.Done()
		}()
	}

	finish := make(chan struct{})
	var results []FileEntry
	go func() {
		for {
			select {
			case fileentry := <-entries:
				results = append(results, fileentry)
			case <-finish:
				collect <- results
				return
			}
		}
	}()

	go func() {
		statWg.Wait()
		close(finish)
		searchWg.Done()
	}()
}

func Search(liststore *gtk.ListStore) {
	updatedb, updatedbErr := exec.LookPath("updatedb.mlocate")
	if updatedbErr != nil {
		panic(updatedbErr)
	}

	mlocate, mlocateErr := exec.LookPath("mlocate")
	if mlocateErr != nil {
		panic(mlocateErr)
	}

	userDirectories := []string{os.Getenv("HOME"), "/usr", "/var"}
	userPrunenames := ""
	userPrunepaths := "/tmp /var/spool /media /home/.ecryptfs /var/lib/schroot"

	splittedDirectories, additionalPrunepaths := SplitDirectories(userDirectories)
	log.Println(additionalPrunepaths)

	collect := make(chan []FileEntry)

	var wg sync.WaitGroup
	query := ".*"
	for _, dir := range splittedDirectories {
		wg.Add(1)
		go SearchDirectory(&wg, collect, updatedb, userPrunenames, userPrunepaths, mlocate, dir, query)
	}

	for _, dir := range userDirectories {
		wg.Add(1)
		go SearchDirectory(&wg, collect, updatedb, userPrunenames, userPrunepaths+additionalPrunepaths, mlocate, dir, query)
	}

	finish := make(chan struct{})
	var sortedResults []FileEntry
	go func() {
		for {
			select {
			case directoryResults := <-collect:
				sortedResults = append(sortedResults, directoryResults...)
			case <-finish:
				log.Println(len(sortedResults))
				return
			}
		}
	}()

	wg.Wait()
	close(finish)
}
