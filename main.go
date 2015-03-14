package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
)

func main() {

	flag.Usage = func() {
		fmt.Fprintf(
			os.Stderr,
			"  Usage: %s file [file file ...]\n",
			path.Base(os.Args[0]),
		)
		//flag.PrintDefaults()
	}

	for _, arg := range os.Args[1:] {
		raw, err := ioutil.ReadFile(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to open %s: %s", arg, err)
			os.Exit(1)
		}
		l := lex(arg, string(raw))
		for i := l.nextItem(); i.typ != itemEOF; i = l.nextItem() {
			fmt.Printf("%#v\n", i)
			if i.typ == itemError {
				fmt.Fprintf(os.Stderr, "Aborting %s at line %d, pos %d\n", arg, l.lineNumber(), l.pos)
				break
			}
		}
	}

}
