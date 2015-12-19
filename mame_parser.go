package main

import (
	"bufio"
	"encoding/xml"
	"io"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"
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

// decodeXMLStream converts supported XML nodes to machine structs
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
						log.Fatalln(err)
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

// convert the XML element machine into a logical Game for the index
func machineToGame(m machine) Game {
	intYear, err := strconv.Atoi(strings.Replace(m.Year, "?", "0", -1))
	if err != nil {
		log.Fatalln(err)
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

// streamXMLChunk parses the machine XML for a given prefix from the MAME binary
func streamXMLChunk(prefix string, results chan Game) {
	//defer timeTrack(time.Now(), "readChunk "+prefix)

	args := []string{"-rompath", *romsPath, "-lx", prefix}
	cmd := exec.Command(*mameBinary, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatalln(err)
	}
	if err := cmd.Start(); err != nil {
		log.Fatalln(err)
	}

	for g := range decodeXMLStream(stdout) {
		results <- g
	}

	if err := cmd.Wait(); err != nil {
		log.Fatalln(err)
	}
}

// listGamePrefixes buckets the first numChars of all supported MAME
// games and returns them as wildcards for listing XML in chunks
func listGamePrefixes(numChars int) (int, <-chan string) {
	output := make(chan string)

	prefixUsed := make(map[string]bool)
	args := []string{"-rompath", *romsPath, "-ll"}
	cmd := exec.Command(*mameBinary, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatalln(err)
	}
	if err := cmd.Start(); err != nil {
		log.Fatalln(err)
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
		log.Fatalln(err)
	}

	if err := cmd.Wait(); err != nil {
		log.Fatalln(err)
	}

	go func() {
		for prefix := range prefixUsed {
			output <- prefix + "*"
		}
		close(output)
	}()

	return len(prefixUsed), output
}
