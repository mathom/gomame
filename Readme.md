# gomame
A basic frontend for MAME written in Go.

## Installation
`go get github.com/mathom/gomame`

## Setup
You'll need to build the search index first:
`gomame -mame path/to/mamebinary -reindex`

The output will look something like this:
```
2015/12/18 05:37:01 Detected 8 cores and 8 Go processes
258 / 689 [====================>----------------------------------] 37.45 % 10s
```

## Searching
You can do full text searches using the
[Bleve query language](http://www.blevesearch.com/docs/Query-String-Query/).

For example:
```
gomame -search 'burger time'
9 matches, showing 1 through 5, took 1.0009ms
    1. bbtime (1.772874)
        Year
                1983
        Manufacturer
                Bandai
        Description
                Burger Time (Bandai)
        DriverStatus
                preliminary
    2. supbtimej (1.667459)
        Year
                1990
        Manufacturer
                Data East Corporation
        Description
                Super Burger Time (Japan)
        DriverStatus
                good
    3. cbtime (1.667459)
        Year
                1983
        Manufacturer
                Data East Corporation
        Description
                Burger Time (DECO Cassette)
        DriverStatus
                good
    4. btimem (1.636289)
        DriverStatus
                good
        Year
                1982
        Manufacturer
                Data East (Bally Midway license)
        Description
                Burger Time (Midway)
    5. supbtimea (1.606804)
        Manufacturer
                Data East Corporation
        Description
                Super Burger Time (World, set 2)
        DriverStatus
                good
        Year
                1990
```
