package main

import (
	"flag"
	"log"

	"github.com/yuhaiin/purego-sqlite-embed/internal/assetgen"
)

func main() {
	source := flag.String("source", "assets", "directory containing verified native candidates")
	flag.Parse()
	if err := assetgen.Install(".", *source, flag.Args()); err != nil {
		log.Fatal(err)
	}
}
