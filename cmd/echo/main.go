package main

import (
	"flag"
	"fmt"
	"strings"
)

func main() {
	nFlag := flag.Bool("n", false, "do not print the trailing newline")
	flag.Parse()
	print := fmt.Println
	if *nFlag {
		print = fmt.Print
	}
	print(strings.Join(flag.Args(), " "))
}
