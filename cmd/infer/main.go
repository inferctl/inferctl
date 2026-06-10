package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) == 1 {
		fmt.Fprintln(os.Stderr, "infer: no verb specified")
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "infer: verb %q is not implemented yet\n", os.Args[1])
	os.Exit(1)
}
