package main

import (
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"math"
	"os"
	"sync"
	"time"

	"github.com/anthonynsimon/bild/transform"
	"github.com/cornelk/gotokit/log"
	chromath "github.com/jkl1337/go-chromath"
)

// BeadConfig configures a bead color.
type BeadConfig struct {
	R, G, B     uint8
	GreyShade   bool
	Translucent bool
	fluorescent bool
}

type beadMachine struct {
	logger *log.Logger

	colorMatchCache     map[color.Color]string
	colorMatchCacheLock sync.RWMutex
	rgbLabCache         map[color.Color]chromath.Lab
	rgbLabCacheLock     sync.RWMutex
	beadStatsDone       chan struct{}

	labTransformer *chromath.LabTransformer
	rgbTransformer *chromath.RGBTransformer
	beadFillPixel  color.RGBA

	inputFileName   string
	outputFileName  string
	htmlFileName    string
	paletteFileName string

	width          int
	height         int
	boardsWidth    int
	boardsHeight   int
	boardDimension int

	beadStyle   bool
	translucent bool
	fluorescent bool

	noColorMatching bool
	greyScale       bool
	blur            float64
	sharpen         bool
	gamma           float64
	contrast        float64
	brightness      float64
}

func (m *beadMachine) process() error {
	inputImage, err := readImageFile(m.inputFileName)
	if err != nil {
		return fmt.Errorf("reading image file: %w", err)
	}

	imageBounds := inputImage.Bounds()
	m.logger.Info("Image pixels",
		log.Int("width", imageBounds.Dx()),
		log.Int("height", imageBounds.Dy()))

	inputImage = m.applyFilters(inputImage) // apply filters before resizing for better results

	newWidth := m.width
	// resize the image if needed
	if m.boardsWidth > 0 { // a given boards number overrides a possible given pixel number
		newWidth = m.boardsWidth * m.boardDimension
	}

	newHeight := m.height
	if m.boardsHeight > 0 {
		newHeight = m.boardsHeight * m.boardDimension
	}
	resized := false
	if newWidth > 0 || newHeight > 0 {
		if newWidth == 0 {
			dy := float64(newHeight) / float64(inputImage.Bounds().Dy())
			newWidth = int(float64(inputImage.Bounds().Dx()) * dy)
		}
		if newHeight == 0 {
			dx := float64(newWidth) / float64(inputImage.Bounds().Dx())
			newHeight = int(float64(inputImage.Bounds().Dy()) * dx)
		}

		inputImage = transform.Resize(inputImage, newWidth, newHeight, transform.Lanczos)
		resized = true
	}
	imageBounds = inputImage.Bounds()
	if imageBounds.Dx() == 0 || imageBounds.Dy() == 0 {
		m.logger.Fatal("An image dimension is 0")
	}

	m.logger.Info("Bead board used",
		log.Int("width", calculateBeadBoardsNeeded(imageBounds.Dx())),
		log.Int("height", calculateBeadBoardsNeeded(imageBounds.Dy())))
	m.logger.Info("Bead board measurement in cm",
		log.Float64("width", float64(imageBounds.Dx())*0.5),
		log.Float64("height", float64(imageBounds.Dy())*0.5))

	beadModeImageBounds := imageBounds
	if m.beadStyle { // each pixel will be a bead of 8x8 pixel
		beadModeImageBounds.Max.X *= 8
		beadModeImageBounds.Max.Y *= 8
	}
	outputImage := image.NewRGBA(beadModeImageBounds)

	if resized || m.beadStyle {
		m.logger.Info("Output image pixels",
			log.Int("width", imageBounds.Dx()),
			log.Int("height", imageBounds.Dy()))
	}

	if m.noColorMatching {
		for y := imageBounds.Min.Y; y < imageBounds.Max.Y; y++ {
			for x := imageBounds.Min.X; x < imageBounds.Max.X; x++ {
				pixelColor := inputImage.At(x, y)
				r, g, b, _ := pixelColor.RGBA()
				pixelRGBA := color.RGBA{uint8(r), uint8(g), uint8(b), 255} // A 255 = no transparency
				outputImage.SetRGBA(x, y, pixelRGBA)
			}
		}
	} else {
		startTime := time.Now()
		if err := m.processImage(imageBounds, inputImage, outputImage); err != nil {
			return fmt.Errorf("processing image: %w", err)
		}
		elapsedTime := time.Since(startTime)
		m.logger.Info("Image processed", log.Duration("duration", elapsedTime))
	}

	imageWriter, err := os.Create(m.outputFileName)
	if err != nil {
		return fmt.Errorf("opening output image file: %w", err)
	}
	defer imageWriter.Close()

	if err = png.Encode(imageWriter, outputImage); err != nil {
		return fmt.Errorf("encoding png file: %w", err)
	}
	return nil
}

// calculateBeadUsage calculates the bead usage statistics.
func (m *beadMachine) calculateBeadUsage(beadUsageChan <-chan string) {
	colorUsageCounts := make(map[string]int)

	for beadName := range beadUsageChan {
		colorUsageCounts[beadName]++
	}

	m.logger.Info("Bead colors", log.Int("count", len(colorUsageCounts)))
	for usedColor, count := range colorUsageCounts {
		m.logger.Info("Beads used", log.String("color", usedColor), log.Int("count", count))
	}
	m.beadStatsDone <- struct{}{}
}

// calculateBeadBoardsNeeded calculates the needed bead boards based on the standard size of 29 beads for a dimension.
func calculateBeadBoardsNeeded(dimension int) int {
	neededFloat := float64(dimension) / 29
	neededFloat = math.Floor(neededFloat + .5)
	return int(neededFloat) // round up
}
