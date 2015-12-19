package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/blevesearch/bleve"
	_ "github.com/blevesearch/bleve/index/store/goleveldb"
	"github.com/vharitonsky/iniflags"
)

var (
	mameBinary = flag.String("mame", "mame/mame", "Path to MAME binary")
	romsPath   = flag.String("roms", "mame/roms", "Path to MAME ROMs")
	indexFile  = flag.String("index", "index.bleve", "File to store search index in")
	reindex    = flag.Bool("reindex", false, "Reindex ROMS")
	debug      = flag.Bool("debug", false, "Print debug information")
	search     = flag.String("search", "", "Fulltext ROM search")
)

func main() {
	iniflags.Parse()

	bleve.Config.DefaultKVStore = "goleveldb"

	if *reindex {
		DeleteIndex()
		IndexRoms()
	}

	if *search != "" {
		index := OpenIndexFile(*indexFile)
		query := bleve.NewQueryStringQuery(*search)
		search := bleve.NewSearchRequest(query)
		search.Fields = []string{"Year", "Manufacturer",
			"Description", "DriverStatus"}
		search.Size = 5
		searchResults, err := index.Search(search)
		if err != nil {
			log.Fatalln(err)
		}
		fmt.Println(searchResults)
	}
}
