package main

import (
	"log"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/blevesearch/bleve"
	"github.com/cheggaaa/pb"
)

// Game data for search index
type Game struct {
	Name         string
	Description  string
	Year         string
	Timestamp    time.Time
	Manufacturer string
	DriverStatus string
}

func timeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	if *debug {
		log.Printf("%s took %s", name, elapsed)
	}
}

// indexGames batches Games and stores them in the search index
func indexGames(index bleve.Index, input <-chan Game) <-chan Game {
	out := make(chan Game)

	maxBatched := 500

	go func() {
		defer timeTrack(time.Now(), "indexGames")
		batch := index.NewBatch()
		count := 0
		for g := range input {
			batch.Index(g.Name, g)
			out <- g
			count++
			if count > maxBatched {
				if err := index.Batch(batch); err != nil {
					log.Fatalln(err)
				}
				batch = index.NewBatch()
				count = 0
			}
		}
		if err := index.Batch(batch); err != nil {
			log.Fatalln(err)
		}
		close(out)
	}()

	return out
}

// OpenIndexFile loads the search index at the specified filename
func OpenIndexFile(filename string) bleve.Index {
	index, err := bleve.Open(filename)
	if err == bleve.ErrorIndexPathDoesNotExist {
		indexMapping := bleve.NewIndexMapping()
		index, err = bleve.New(filename, indexMapping)
		if err != nil {
			log.Fatalln(err)
		}
	} else if err != nil {
		log.Fatalln(err)
	}
	return index
}

// indexWorker takes prefixes from a channel and parses the XML for each
func indexWorker(prefixes <-chan string, prefixFinished func()) <-chan Game {
	output := make(chan Game)
	go func() {
		for p := range prefixes {
			streamXMLChunk(p, output)
			prefixFinished()
		}
		close(output)
	}()
	return output
}

// merge the given channels into one stream and synchronize at the end
func merge(cs []<-chan Game) <-chan Game {
	// see https://blog.golang.org/pipelines
	var wg sync.WaitGroup
	out := make(chan Game)

	output := func(c <-chan Game) {
		for n := range c {
			out <- n
		}
		wg.Done()
	}
	wg.Add(len(cs))
	for _, c := range cs {
		go output(c)
	}

	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

// IndexRoms builds a complete game index from MAME's XML data
func IndexRoms() {
	defer timeTrack(time.Now(), "indexRoms")

	prefixCount, prefixes := listGamePrefixes(2)
	index := OpenIndexFile(*indexFile)

	log.Printf("Detected %d cores and %d Go processes",
		runtime.NumCPU(), runtime.GOMAXPROCS(0))

	bar := pb.StartNew(prefixCount)

	prefixFinished := func() {
		bar.Increment()
	}

	numWorkers := runtime.GOMAXPROCS(0)

	var workers []<-chan Game
	for i := 0; i < numWorkers; i++ {
		workers = append(workers, indexWorker(prefixes, prefixFinished))
	}

	for range indexGames(index, merge(workers)) {
	}
	bar.FinishPrint("Indexing complete!")
}

// DeleteIndex erases the index store on disk
func DeleteIndex() {
	log.Printf("Removing index at %s", *indexFile)
	if err := os.RemoveAll(*indexFile); err != nil {
		log.Fatalln(err)
	}
}
