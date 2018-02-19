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
	"time"

	"github.com/go-cmd/cmd"
	"github.com/gotk3/gotk3/glib"
	"gotk3/gtk"
)

type RowEntry struct {
	fileinfo os.FileInfo
	score    int
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

func RunLocateCommand(dbname, mlocate, dir string) []string {
	mlocateCmd := cmd.NewCmd(mlocate, "-d", "/dev/shm/"+dbname+".db", "-r", ".*")
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

func SearchDirectory(wg *sync.WaitGroup, liststore *gtk.ListStore, updatedb, prunenames, prunepaths, mlocate, dir string) {
	dbname := DirToDbName(dir)
	RunUpdatedbCommand(dbname, updatedb, prunenames, prunepaths, dir)
	stdout := RunLocateCommand(dbname, mlocate, dir)
	log.Println(dir, ": ", len(stdout))
	for i := 0; i < len(stdout); i++ {
		fileinfo, err := os.Stat(stdout[i])
		if err != nil {
			log.Println("Error reading file:", err)
		} else {
			t := fileinfo.ModTime()
			glib.IdleAdd(AddRow, liststore, stdout[i], t.Format(time.UnixDate))
		}
	}
	defer wg.Done()
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

	userDirectories := []string{os.Getenv("HOME")}
	userPrunenames := ".git .bzr .hg .svn"
	userPrunepaths := "/tmp /var/spool /media /home/.ecryptfs /var/lib/schroot"

	splittedDirectories, additionalPrunepaths := SplitDirectories(userDirectories)
	log.Println(additionalPrunepaths)

	var wg sync.WaitGroup
	for _, dir := range splittedDirectories {
		wg.Add(1)
		SearchDirectory(&wg, liststore, updatedb, userPrunenames, userPrunepaths, mlocate, dir)
	}

	for _, dir := range userDirectories {
		wg.Add(1)
		SearchDirectory(&wg, liststore, updatedb, userPrunenames, userPrunepaths+additionalPrunepaths, mlocate, dir)
	}
	wg.Wait()
}
