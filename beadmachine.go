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
	_ "image/png"

	chromath "github.com/jkl1337/go-chromath"
	"github.com/jkl1337/go-chromath/deltae"
	kingpin "gopkg.in/alecthomas/kingpin.v1"
)

// RGB is a simple RGB struct to define a color
type RGB struct {
	R, G, B uint8
}

var (
	useColorMatchCache = kingpin.Flag("cache", "Cache result of each matched color.").Short('c').Bool()
	inputFileName      = kingpin.Flag("input", "Filename of image to process.").Short('i').Required().String()
	outputFileName     = kingpin.Flag("output", "Output filename for the converted PNG image.").Short('o').PlaceHolder("OUTPUT.png").Required().String()
	paletteFileName    = kingpin.Flag("palette", "Filename of the bead palette.").Short('p').Default("colors_hama.json").String()

	targetIlluminant = &chromath.IlluminantRefD50
	labTransformer   = chromath.NewLabTransformer(targetIlluminant)
	rgbTransformer   = chromath.NewRGBTransformer(&chromath.SpaceSRGB, &chromath.AdaptationBradford, targetIlluminant, &chromath.Scaler8bClamping, 1.0, nil)

	colorMatchCache     = make(map[color.Color]string)
	colorMatchCacheLock sync.RWMutex
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
func FindSimilarColor(cfgLab map[chromath.Lab]string, pixel color.Color) string {
	if *useColorMatchCache {
		colorMatchCacheLock.RLock()
		match, found := colorMatchCache[pixel]
		colorMatchCacheLock.RUnlock()
		if found {
			return match
		}
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
	return bestBeadMatch
}

func main() {
	kingpin.CommandLine.Help = "Bead pattern creator."
	kingpin.Parse()

	runtime.GOMAXPROCS(runtime.NumCPU()) // use all cores for concurrency

	beadConfig, beadLab := LoadPalette(*paletteFileName)

	imageReader, err := os.Open(*inputFileName)
	if err != nil {
		fmt.Printf("Opening input image file failed: %v\n", err)
		os.Exit(1)
	}
	defer imageReader.Close()

	inputImage, _, err := image.Decode(imageReader)
	inputBounds := inputImage.Bounds()
	outputImage := image.NewRGBA(inputBounds)

	startTime := time.Now()

	beadUsageSplice := make([]string, inputBounds.Dx()*inputBounds.Dy())
	var wg sync.WaitGroup
	for y := inputBounds.Min.Y; y < inputBounds.Max.Y; y++ {
		for x := inputBounds.Min.X; x < inputBounds.Max.X; x++ {
			wg.Add(1)
			go func(x int, y int) {
				defer wg.Done()
				oldPixel := inputImage.At(x, y)
				beadName := FindSimilarColor(beadLab, oldPixel)
				beadUsageSplice[(y*inputBounds.Dx())+x] = beadName // race and lock free accounting of used color
				matchRgb := beadConfig[beadName]
				rgbaMatch := color.RGBA{matchRgb.R, matchRgb.G, matchRgb.B, 255} // A 255 = no transparency

				if *useColorMatchCache {
					colorMatchCacheLock.Lock()
					colorMatchCache[oldPixel] = beadName
					colorMatchCacheLock.Unlock()
				}
				outputImage.SetRGBA(x, y, rgbaMatch)
			}(x, y)
		}
	}
	wg.Wait() // wait for all pixels to be processed

	elapsedTime := time.Since(startTime)
	fmt.Printf("Image processed in %s\n", elapsedTime)

	colorUsageCounts := make(map[string]int)
	for _, usedColor := range beadUsageSplice {
		colorUsageCounts[usedColor]++
	}

	fmt.Printf("Bead colors used: %v\n", len(colorUsageCounts))
	for usedColor, count := range colorUsageCounts {
		fmt.Printf("Beads used for color '%s': %v\n", usedColor, count)
	}

	imageWriter, err := os.Create(*outputFileName)
	if err != nil {
		fmt.Printf("Opening output image file failed: %v\n", err)
		os.Exit(1)
	}
	defer imageWriter.Close()

	png.Encode(imageWriter, outputImage)
}
