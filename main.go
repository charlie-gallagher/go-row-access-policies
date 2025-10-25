package main

import (
	"fmt"
	"github.com/spf13/pflag"
)

func main() {
	var verbose bool
	var output string

	pflag.BoolVarP(&verbose, "verbose", "v", false, "enable verbose mode")
	pflag.StringVarP(&output, "output", "o", "", "output file")

	pflag.Parse()

	fmt.Println("Verbose:", verbose)
	fmt.Println("Output:", output)
	fmt.Println("Args:", pflag.Args())
}
