package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/ioutil"
	"math"
	"os"
	"runtime"
	"sync"
	"time"

	_ "image/gif"
	_ "image/jpeg"

	"github.com/disintegration/imaging"
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
	newWidth        = kingpin.Flag("width", "Resize image to width.").Short('w').Default("0").Int()
	newHeight       = kingpin.Flag("height", "Resize image to height.").Short('h').Default("0").Int()
	beadStyle       = kingpin.Flag("bead", "Make output file look like a beads board.").Short('b').Bool()
	greyScale       = kingpin.Flag("grey", "Convert the image to greyscale.").Short('g').Bool()

	targetIlluminant = &chromath.IlluminantRefD50
	labTransformer   = chromath.NewLabTransformer(targetIlluminant)
	rgbTransformer   = chromath.NewRGBTransformer(&chromath.SpaceSRGB, &chromath.AdaptationBradford, targetIlluminant, &chromath.Scaler8bClamping, 1.0, nil)
	beadFillPixel    = color.RGBA{225, 225, 225, 255} // light grey

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
		fmt.Printf("Beads used '%s': %v\n", usedColor, count)
	}
	beadStatsDone <- struct{}{}
}

// setOutputImagePixel sets a pixel in the output image or draws a bead in beadStyle mode
func setOutputImagePixel(outputImage *image.RGBA, coordinates image.Point, newRGB RGB) {
	rgbaMatch := color.RGBA{newRGB.R, newRGB.G, newRGB.B, 255} // A 255 = no transparency
	if *beadStyle {
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				if (x%7 == 0 && y%7 == 0) || (x > 2 && x < 5 && y > 2 && y < 5) { // all corner pixel + 2x2 in center
					outputImage.SetRGBA((coordinates.X*8)+x, (coordinates.Y*8)+y, beadFillPixel)
				} else {
					outputImage.SetRGBA((coordinates.X*8)+x, (coordinates.Y*8)+y, rgbaMatch)
				}
			}
		}
	} else {
		outputImage.SetRGBA(coordinates.X, coordinates.Y, rgbaMatch)
	}
}

// calculateBeadBoardsNeeded calculates the needed bead boards based on the standard size of 29 beads for a dimension
func calculateBeadBoardsNeeded(dimension int) int {
	neededFloat := float64(dimension) / 29
	neededFloat = math.Floor(neededFloat + .5)
	return int(neededFloat) // round up
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
	imageBounds := inputImage.Bounds()
	fmt.Println("Input image width:", imageBounds.Dx(), "height:", imageBounds.Dy())

	if *greyScale { // better looking results when doing before a possible resize
		inputImage = imaging.Grayscale(inputImage)
	}

	resized := false
	if *newWidth > 0 || *newHeight > 0 {
		inputImage = imaging.Resize(inputImage, *newWidth, *newHeight, imaging.Lanczos)
		imageBounds = inputImage.Bounds()
		resized = true
	}
	pixelCount := imageBounds.Dx() * imageBounds.Dy()

	outputImageBounds := imageBounds
	if resized || *beadStyle {
		fmt.Println("Beads width:", outputImageBounds.Dx(), "height:", outputImageBounds.Dy())
	}
	fmt.Println("Bead boards width:", calculateBeadBoardsNeeded(outputImageBounds.Dx()), "height:", calculateBeadBoardsNeeded(outputImageBounds.Dy()))

	if *beadStyle { // each pixel will be a bead of 8x8 pixel
		outputImageBounds.Max.X *= 8
		outputImageBounds.Max.Y *= 8
	}
	if resized || *beadStyle {
		fmt.Println("Output image width:", outputImageBounds.Dx(), "height:", outputImageBounds.Dy())
	}
	outputImage := image.NewRGBA(outputImageBounds)

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
					if cached == false {
						colorMatchCacheLock.Lock()
						colorMatchCache[oldPixel] = beadName
						colorMatchCacheLock.Unlock()
					}

					matchRgb := beadConfig[beadName]
					setOutputImagePixel(outputImage, pixel, matchRgb)
				}(pixel)
			case <-workDone:
				return
			}
		}
	}()

	go calculateBeadUsage(beadUsageChan)

	startTime := time.Now()
	for y := imageBounds.Min.Y; y < imageBounds.Max.Y; y++ {
		for x := imageBounds.Min.X; x < imageBounds.Max.X; x++ {
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
