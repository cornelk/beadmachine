package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/ioutil"
	"os"
	"runtime"
	"sync"
	"time"

	_ "image/gif"
	_ "image/jpeg"

	chromath "github.com/jkl1337/go-chromath"
	"github.com/jkl1337/go-chromath/deltae"
	kingpin "gopkg.in/alecthomas/kingpin.v1"
)

// RGB is a simple RGB struct to define a color
type RGB struct {
	R, G, B uint8
}

var (
	inputFileName   = kingpin.Flag("input", "Filename of image to process.").Short('i').Required().String()
	outputFileName  = kingpin.Flag("output", "Output filename for the converted PNG image.").Short('o').PlaceHolder("OUTPUT.png").Required().String()
	paletteFileName = kingpin.Flag("palette", "Filename of the bead palette.").Short('p').Default("colors_hama.json").String()

	targetIlluminant = &chromath.IlluminantRefD50
	labTransformer   = chromath.NewLabTransformer(targetIlluminant)
	rgbTransformer   = chromath.NewRGBTransformer(&chromath.SpaceSRGB, &chromath.AdaptationBradford, targetIlluminant, &chromath.Scaler8bClamping, 1.0, nil)

	colorMatchCache     = make(map[color.Color]string)
	colorMatchCacheLock sync.RWMutex
	beadStatsDone       = make(chan struct{})
)

// LoadPalette loads a palette from a json file and returns a LAB color palette
func LoadPalette(fileName string) (map[string]RGB, map[chromath.Lab]string) {
	cfgData, err := ioutil.ReadFile(fileName)
	if err != nil {
		fmt.Printf("File error: %v\n", err)
		os.Exit(1)
	}

	cfg := make(map[string]RGB)
	err = json.Unmarshal(cfgData, &cfg)
	if err != nil {
		fmt.Printf("Config json error: %v\n", err)
		os.Exit(1)
	}

	cfgLab := make(map[chromath.Lab]string)
	for beadName, rgbOriginal := range cfg {
		rgb := chromath.RGB{float64(rgbOriginal.R), float64(rgbOriginal.G), float64(rgbOriginal.B)}
		xyz := rgbTransformer.Convert(rgb)
		lab := labTransformer.Invert(xyz)
		cfgLab[lab] = beadName
		//fmt.Println("Loaded bead:", beadName, "RGB:", rgb, "Lab:", lab)
	}

	return cfg, cfgLab
}

// FindSimilarColor finds the most similar color from bead palette to the given pixel
func FindSimilarColor(cfgLab map[chromath.Lab]string, pixel color.Color) (string, bool) {
	colorMatchCacheLock.RLock()
	match, found := colorMatchCache[pixel]
	colorMatchCacheLock.RUnlock()
	if found {
		return match, true
	}

	r, g, b, _ := pixel.RGBA()
	rgb := chromath.RGB{float64(uint8(r)), float64(uint8(g)), float64(uint8(b))}
	xyz := rgbTransformer.Convert(rgb)
	labPixel := labTransformer.Invert(xyz)

	var minDistance float64
	var bestBeadMatch string

	for lab, beadName := range cfgLab {
		distance := deltae.CIE2000(lab, labPixel, &deltae.KLChDefault)
		if len(bestBeadMatch) == 0 || distance < minDistance {
			minDistance = distance
			bestBeadMatch = beadName
		}
		//fmt.Println("Match:", beadName, "with distance:", distance)
	}

	//fmt.Println("Best match:", bestBeadMatch, "with distance:", minDistance)
	return bestBeadMatch, false
}

// calculateBeadUsage calculates the bead usage
func calculateBeadUsage(beadUsageChan <-chan string) {
	colorUsageCounts := make(map[string]int)

	for beadName := range beadUsageChan {
		colorUsageCounts[beadName]++
	}

	fmt.Printf("Bead colors used: %v\n", len(colorUsageCounts))
	for usedColor, count := range colorUsageCounts {
		fmt.Printf("Beads used for color '%s': %v\n", usedColor, count)
	}
	beadStatsDone <- struct{}{}
}

func main() {
	kingpin.CommandLine.Help = "Bead pattern creator."
	kingpin.Parse()

	cpuCount := runtime.NumCPU()
	runtime.GOMAXPROCS(cpuCount) // use all cores for parallelism

	beadConfig, beadLab := LoadPalette(*paletteFileName)

	imageReader, err := os.Open(*inputFileName)
	if err != nil {
		fmt.Printf("Opening input image file failed: %v\n", err)
		os.Exit(1)
	}
	defer imageReader.Close()

	inputImage, _, err := image.Decode(imageReader)
	inputBounds := inputImage.Bounds()
	pixelCount := inputBounds.Dx() * inputBounds.Dy()
	fmt.Println("Image width:", inputBounds.Dx(), "height:", inputBounds.Dy())

	outputImage := image.NewRGBA(inputBounds)

	beadUsageChan := make(chan string, pixelCount)
	workQueueChan := make(chan image.Point, cpuCount*2)
	workDone := make(chan struct{})

	var pixelWaitGroup sync.WaitGroup
	pixelWaitGroup.Add(pixelCount)
	go func() { // pixel channel worker goroutine
		for {
			select {
			case pixel := <-workQueueChan:
				go func(pixel image.Point) { // pixel processing goroutine
					defer pixelWaitGroup.Done()
					oldPixel := inputImage.At(pixel.X, pixel.Y)
					beadName, cached := FindSimilarColor(beadLab, oldPixel)
					beadUsageChan <- beadName
					matchRgb := beadConfig[beadName]
					rgbaMatch := color.RGBA{matchRgb.R, matchRgb.G, matchRgb.B, 255} // A 255 = no transparency

					if cached == false {
						colorMatchCacheLock.Lock()
						colorMatchCache[oldPixel] = beadName
						colorMatchCacheLock.Unlock()
					}
					outputImage.SetRGBA(pixel.X, pixel.Y, rgbaMatch)
				}(pixel)
			case <-workDone:
				return
			}
		}
	}()

	go calculateBeadUsage(beadUsageChan)

	startTime := time.Now()
	for y := inputBounds.Min.Y; y < inputBounds.Max.Y; y++ {
		for x := inputBounds.Min.X; x < inputBounds.Max.X; x++ {
			workQueueChan <- image.Point{x, y}
		}
	}

	pixelWaitGroup.Wait() // wait for all pixel to be processed
	workDone <- struct{}{}
	close(workQueueChan)
	close(beadUsageChan)
	<-beadStatsDone

	elapsedTime := time.Since(startTime)
	fmt.Printf("Image processed in %s\n", elapsedTime)

	imageWriter, err := os.Create(*outputFileName)
	if err != nil {
		fmt.Printf("Opening output image file failed: %v\n", err)
		os.Exit(1)
	}
	defer imageWriter.Close()

	png.Encode(imageWriter, outputImage)
}
