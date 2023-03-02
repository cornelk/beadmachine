package main

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"runtime"
	"sync"

	"github.com/disintegration/imaging"
)

// readImageFile reads and decodes the given image file.
func readImageFile(fileName string) (image.Image, error) {
	imageReader, err := os.Open(fileName)
	if err != nil {
		return nil, fmt.Errorf("opening image file: %w", err)
	}
	defer imageReader.Close()

	inputImage, _, err := image.Decode(imageReader)
	if err != nil {
		return nil, fmt.Errorf("decoding image file: %w", err)
	}

	return inputImage, nil
}

// processImage matches all pixel of the image to a matching bead.
func (m *beadMachine) processImage(imageBounds image.Rectangle, inputImage image.Image, outputImage *image.RGBA) error {
	beadConfig, beadLab, err := m.loadPalette()
	if err != nil {
		return err
	}

	pixelCount := imageBounds.Dx() * imageBounds.Dy()
	beadUsageChan := make(chan string, pixelCount)
	workQueueChan := make(chan image.Point, runtime.NumCPU()*2)
	workDone := make(chan struct{})

	var outputImageBeadNames []string // TODO use pointer to bead config instead of string
	if m.htmlFileName != "" {
		outputImageBeadNames = make([]string, pixelCount)
	}

	var pixelWaitGroup sync.WaitGroup
	pixelWaitGroup.Add(pixelCount)

	go func() { // pixel channel worker goroutine
		for {
			select {
			case pixel := <-workQueueChan:
				go func(pixel image.Point) { // pixel processing goroutine
					defer pixelWaitGroup.Done()
					oldPixel := inputImage.At(pixel.X, pixel.Y)
					beadName := m.findSimilarColor(beadLab, oldPixel)
					beadUsageChan <- beadName

					if m.htmlFileName != "" {
						outputImageBeadNames[pixel.X+pixel.Y*imageBounds.Max.X] = beadName
					}

					matchRgb := beadConfig[beadName]
					m.setOutputImagePixel(outputImage, pixel, matchRgb)
				}(pixel)
			case <-workDone:
				return
			}
		}
	}()

	go m.calculateBeadUsage(beadUsageChan)

	for y := imageBounds.Min.Y; y < imageBounds.Max.Y; y++ {
		for x := imageBounds.Min.X; x < imageBounds.Max.X; x++ {
			workQueueChan <- image.Point{
				X: x,
				Y: y,
			}
		}
	}

	pixelWaitGroup.Wait() // wait for all pixel to be processed
	workDone <- struct{}{}
	close(workQueueChan)
	close(beadUsageChan)
	<-m.beadStatsDone

	if m.htmlFileName != "" {
		return m.writeHTMLBeadInstructionFile(imageBounds, outputImage, outputImageBeadNames)
	}
	return nil
}

// applyFilters will apply all filters that were enabled to the input image.
func (m *beadMachine) applyFilters(inputImage image.Image) image.Image {
	filteredImage := inputImage

	if m.greyScale {
		filteredImage = imaging.Grayscale(filteredImage)
	}
	if m.blur != 0.0 {
		filteredImage = imaging.Blur(filteredImage, m.blur)
	}
	if m.sharpen != 0.0 {
		filteredImage = imaging.Sharpen(filteredImage, m.sharpen)
	}
	if m.gamma != 0.0 {
		filteredImage = imaging.AdjustGamma(filteredImage, m.gamma)
	}
	if m.contrast != 0.0 {
		filteredImage = imaging.AdjustContrast(filteredImage, m.contrast)
	}
	if m.brightness != 0.0 {
		filteredImage = imaging.AdjustBrightness(filteredImage, m.brightness)
	}

	return filteredImage
}

// setOutputImagePixel sets a pixel in the output image or draws a bead in beadStyle mode.
func (m *beadMachine) setOutputImagePixel(outputImage *image.RGBA, coordinates image.Point, bead BeadConfig) {
	rgbaMatch := color.RGBA{
		R: bead.R,
		G: bead.G,
		B: bead.B,
		A: 255, // A 255 = no transparency
	}
	if !m.beadStyle {
		outputImage.SetRGBA(coordinates.X, coordinates.Y, rgbaMatch)
		return
	}

	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			if (x%7 == 0 && y%7 == 0) || (x > 2 && x < 5 && y > 2 && y < 5) { // all corner pixel + 2x2 in center
				outputImage.SetRGBA((coordinates.X*8)+x, (coordinates.Y*8)+y, m.beadFillPixel)
			} else {
				outputImage.SetRGBA((coordinates.X*8)+x, (coordinates.Y*8)+y, rgbaMatch)
			}
		}
	}
}
