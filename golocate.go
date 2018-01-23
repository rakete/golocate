package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/go-cmd/cmd"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

func DirToDbName(dir string) string {
	name := strings.Replace(strings.Trim(dir, "/"), "/", "-", -1)
	if len(name) > 0 {
		name = "-" + name
	}
	name = "golocate" + name
	return name
}

func UpdateDbAndLocate(dir string) {
	mlocate, lookErr := exec.LookPath("updatedb.mlocate")
	if lookErr != nil {
		panic(lookErr)
	}

	dbname := DirToDbName(dir)

	cmd := cmd.NewCmd(mlocate, "--require-visibility", "0", "-o", "/home/rakete/mlocate/"+dbname+".db", "-U", dir)
	statusChan := cmd.Start()

	<-statusChan
	fmt.Println(dir)
}

func Search() {
	directories := []string{"/home/rakete", "/usr", "/var", "/bin", "/lib", "/sys", "/proc"}

	var wg sync.WaitGroup
	for _, dir := range directories {
		wg.Add(1)
		go func(dir string) {
			UpdateDbAndLocate(dir)
			defer wg.Done()
		}(dir)
	}

	wg.Wait()
}

func main() {
	// Create Gtk Application, change appID to your application domain name reversed.
	const appID = "org.gtk.example"
	application, err := gtk.ApplicationNew(appID, glib.APPLICATION_FLAGS_NONE)
	// Check to make sure no errors when creating Gtk Application
	if err != nil {
		log.Fatal("Could not create application.", err)
	}

	// Application signals available
	// startup -> sets up the application when it first starts
	// activate -> shows the default first window of the application (like a new document). This corresponds to the application being launched by the desktop environment.
	// open -> opens files and shows them in a new window. This corresponds to someone trying to open a document (or documents) using the application from the file browser, or similar.
	// shutdown ->  performs shutdown tasks
	// Setup activate signal with a closure function.
	application.Connect("activate", func() {
		// Create ApplicationWindow
		appWindow, err := gtk.ApplicationWindowNew(application)
		if err != nil {
			log.Fatal("Could not create application window.", err)
		}
		// Set ApplicationWindow Properties
		appWindow.SetTitle("Basic Application.")
		appWindow.SetDefaultSize(400, 400)
		appWindow.Show()
	})
	// Run Gtk application
	application.Run(os.Args)
}
