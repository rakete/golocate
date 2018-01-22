package main

import (
	"fmt"
	//"os"
	"os/exec"
	//"path/filepath"
	"strings"
	//"syscall"
)

func DirToDbName(dir string) string {
	name := strings.Replace(strings.Trim(dir, "/"), "/", "-", -1)
	if len(name) > 0 {
		name = "-" + name
	}
	name = "golocate" + name
	return name
}

func UpdateDbAndLocate(dir string) *exec.Cmd {
	mlocate, lookErr := exec.LookPath("updatedb.mlocate")
	if lookErr != nil {
		panic(lookErr)
	}

	name := DirToDbName(dir)

	cmd := exec.Command(mlocate, "--require-visibility", "0", "-o", "/home/rakete/mlocate/"+name+".db", "-U", dir)

	execErr := cmd.Start()

	if execErr != nil {
		panic(execErr)
	}

	return cmd
}

func main() {

	directories := []string{"/home/rakete", "/usr", "/"}

	for _, dir := range directories {
		fmt.Println(dir)
		UpdateDbAndLocate(dir)
	}

}
