package main

import (
	"fmt"
	"github.com/go-cmd/cmd"
	"os/exec"
	"strings"
	"sync"
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

func main() {

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
