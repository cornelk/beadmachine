package main

import (
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"math"
	"os"
	"sync"
	"time"

	"github.com/disintegration/imaging"
	chromath "github.com/jkl1337/go-chromath"
	"go.uber.org/zap"
)

// BeadConfig configures a bead color
type BeadConfig struct {
	R, G, B     uint8
	GreyShade   bool
	Translucent bool
	Flourescent bool
}

type beadMachine struct {
	logger *zap.Logger

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
	flourescent bool

	noColorMatching bool
	greyScale       bool
	blur            float64
	sharpen         float64
	gamma           float64
	contrast        float64
	brightness      float64
}

func (m *beadMachine) process() {
	inputImage, err := readImageFile(m.inputFileName)
	if err != nil {
		m.logger.Error("Reading image file failed", zap.Error(err))
		return
	}

	imageBounds := inputImage.Bounds()
	m.logger.Info("Image pixels",
		zap.Int("width", imageBounds.Dx()),
		zap.Int("height", imageBounds.Dy()))

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
		inputImage = imaging.Resize(inputImage, newWidth, newHeight, imaging.Lanczos)
		imageBounds = inputImage.Bounds()
		resized = true
	}

	m.logger.Info("Bead board used",
		zap.Int("width", calculateBeadBoardsNeeded(imageBounds.Dx())),
		zap.Int("height", calculateBeadBoardsNeeded(imageBounds.Dy())))
	m.logger.Info("Bead board measurement in cm",
		zap.Float64("width", float64(imageBounds.Dx())*0.5),
		zap.Float64("height", float64(imageBounds.Dy())*0.5))

	beadModeImageBounds := imageBounds
	if m.beadStyle { // each pixel will be a bead of 8x8 pixel
		beadModeImageBounds.Max.X *= 8
		beadModeImageBounds.Max.Y *= 8
	}
	outputImage := image.NewRGBA(beadModeImageBounds)

	if resized || m.beadStyle {
		m.logger.Info("Output image pixels",
			zap.Int("width", imageBounds.Dx()),
			zap.Int("height", imageBounds.Dy()))
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
			m.logger.Error("Processing image failed", zap.Error(err))
			return
		}
		elapsedTime := time.Since(startTime)
		m.logger.Info("Image processed", zap.Duration("duration", elapsedTime))
	}

	imageWriter, err := os.Create(m.outputFileName)
	if err != nil {
		m.logger.Error("Opening output image file failed", zap.Error(err))
		return
	}
	defer imageWriter.Close()

	if err = png.Encode(imageWriter, outputImage); err != nil {
		m.logger.Error("Encoding png file failed", zap.Error(err))
	}
}

// calculateBeadUsage calculates the bead usage
func (m *beadMachine) calculateBeadUsage(beadUsageChan <-chan string) {
	colorUsageCounts := make(map[string]int)

	for beadName := range beadUsageChan {
		colorUsageCounts[beadName]++
	}

	m.logger.Info("Bead colors", zap.Int("count", len(colorUsageCounts)))
	for usedColor, count := range colorUsageCounts {
		m.logger.Info("Beads used", zap.String("color", usedColor), zap.Int("count", count))
	}
	m.beadStatsDone <- struct{}{}
}

// calculateBeadBoardsNeeded calculates the needed bead boards based on the standard size of 29 beads for a dimension
func calculateBeadBoardsNeeded(dimension int) int {
	neededFloat := float64(dimension) / 29
	neededFloat = math.Floor(neededFloat + .5)
	return int(neededFloat) // round up
}
