package main

import (
	"flag"
	"fmt"
	"os"
)

var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("logpose", version)
		return
	}

	fmt.Fprintln(os.Stderr, "logpose: nothing to run yet")
	os.Exit(1)
}
