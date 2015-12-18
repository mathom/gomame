package main

import (
	"bufio"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"log"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/blevesearch/bleve"
	"github.com/cheggaaa/pb"
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

type mame struct {
	Build      string `xml:"build,attr"`
	Debug      string `xml:"debug,attr"`
	MameConfig int    `xml:"mameconfig,attr"`
}

type driver struct {
	Status    string `xml:"status,attr"`
	Emulation string `xml:"emulation,attr"`
	Color     string `xml:"color,attr"`
	Sound     string `xml:"sound,attr"`
	Graphic   string `xml:"graphic,attr"`
	SaveState string `xml:"savestate,attr"`
}

type machine struct {
	Name         string `xml:"name,attr"`
	IsBios       string `xml:"isbios,attr"`
	IsDevice     string `xml:"isdevice,attr"`
	IsMechanical string `xml:"ismechanical,attr"`
	Runnable     string `xml:"runnable,attr"`
	CloneOf      string `xml:"cloneof,attr"`
	SampleOf     string `xml:"sampleof,attr"`
	Description  string `xml:"description"`
	Year         string `xml:"year"`
	Manufacturer string `xml:"manufacturer"`
	Driver       driver `xml:"driver"`
}

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

func decodeXMLStream(input io.Reader) <-chan Game {
	out := make(chan Game)
	decoder := xml.NewDecoder(input)

	go func() {
		//defer timeTrack(time.Now(), "decodeXMLStream")
		for {
			t, _ := decoder.Token()
			if t == nil {
				break
			}

			switch se := t.(type) {
			case xml.StartElement:
				if se.Name.Local == "machine" {
					var m machine
					err := decoder.DecodeElement(&m, &se)
					if err != nil {
						log.Fatal(err)
					}
					if m.Runnable == "no" || m.IsBios == "yes" ||
						m.IsDevice == "yes" || m.IsMechanical == "yes" {
						continue // skip non-game machines
					}
					out <- machineToGame(m)
				}
			}
		}
		close(out)
	}()
	return out
}

func machineToGame(m machine) Game {
	intYear, err := strconv.Atoi(strings.Replace(m.Year, "?", "0", -1))
	if err != nil {
		log.Fatal(err)
	}
	return Game{
		Name:         m.Name,
		Description:  m.Description,
		Year:         m.Year,
		Timestamp:    time.Date(intYear, 1, 1, 0, 0, 0, 0, time.UTC),
		Manufacturer: m.Manufacturer,
		DriverStatus: m.Driver.Status,
	}
}

func indexGames(index bleve.Index, input <-chan Game) <-chan Game {
	out := make(chan Game)

	go func() {
		defer timeTrack(time.Now(), "indexGames")
		for g := range input {
			index.Index(g.Name, g)
			out <- g
		}
		close(out)
	}()

	return out
}

func openIndexFile(filename string) bleve.Index {
	index, err := bleve.Open(filename)
	if err == bleve.ErrorIndexPathDoesNotExist {
		indexMapping := bleve.NewIndexMapping()
		index, err = bleve.New(filename, indexMapping)
		if err != nil {
			log.Fatal(err)
		}
	} else if err != nil {
		log.Fatal(err)
	}
	return index
}

func readChunk(prefix string, results chan Game) {
	//defer timeTrack(time.Now(), "readChunk "+prefix)

	args := []string{"-rompath", *romsPath, "-lx", prefix}
	cmd := exec.Command(*mameBinary, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}

	for g := range decodeXMLStream(stdout) {
		results <- g
	}

	if err := cmd.Wait(); err != nil {
		log.Fatal(err)
	}
}

func indexWorker(prefixes <-chan string, prefixFinished func()) <-chan Game {
	output := make(chan Game)
	go func() {
		for p := range prefixes {
			readChunk(p, output)
			prefixFinished()
		}
		close(output)
	}()
	return output
}

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

func listPrefixes(numChars int) (int, <-chan string) {
	output := make(chan string)

	prefixUsed := make(map[string]bool)
	args := []string{"-rompath", *romsPath, "-ll"}
	cmd := exec.Command(*mameBinary, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}

	scanner := bufio.NewScanner(stdout)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		if lineNum == 1 {
			continue // skip first line (it's a header)
		}
		name := strings.SplitAfterN(scanner.Text(), " ", 2)[0]
		prefix := name[0:numChars]
		if _, used := prefixUsed[prefix]; used == false {
			prefixUsed[prefix] = true
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	if err := cmd.Wait(); err != nil {
		log.Fatal(err)
	}

	go func() {
		for prefix := range prefixUsed {
			output <- prefix + "*"
		}
		close(output)
	}()

	return len(prefixUsed), output
}

func indexRoms() {
	defer timeTrack(time.Now(), "indexRoms")

	prefixCount, prefixes := listPrefixes(2)
	index := openIndexFile(*indexFile)

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

func main() {
	iniflags.Parse()

	if *reindex {
		indexRoms()
	}

	if *search != "" {
		index := openIndexFile(*indexFile)
		query := bleve.NewQueryStringQuery(*search)
		search := bleve.NewSearchRequest(query)
		search.Fields = []string{"Year", "Manufacturer",
			"Description", "DriverStatus"}
		search.Size = 5
		searchResults, err := index.Search(search)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(searchResults)
	}
}
