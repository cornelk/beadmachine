package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"io/ioutil"
	"os"
	"strings"

	"github.com/jkl1337/go-chromath"
	"github.com/jkl1337/go-chromath/deltae"
)

// writeHTMLBeadInstructionFile writes a HTML file with instructions on how to make the bead based image
func writeHTMLBeadInstructionFile(fileName string, outputImageBounds image.Rectangle, outputImage *image.RGBA, outputImageBeadNames []string) {
	htmlFile, err := os.Create(fileName)
	if err != nil {
		fmt.Printf("Opening HTML output file failed: %v\n", err)
		os.Exit(1)
	}

	w := bufio.NewWriter(htmlFile)
	w.WriteString("<html>\n<head>\n")
	w.WriteString("<style type=\"text/css\">\n")
	w.WriteString("td { text-align: center }\n")
	w.WriteString(".lb { border-left: 2px solid black !important; }\n")
	w.WriteString(".rb { border-right: 2px solid black !important; }\n")
	w.WriteString(".tb td { border-top: 2px solid black !important; }\n")
	w.WriteString(".bb td { border-bottom: 2px solid black !important; }\n")
	w.WriteString(".bg td:nth-child(even) { background-color: #E0E0E0; }\n")
	w.WriteString("</style>\n</head>\n<body>\n")
	w.WriteString("<table style=\"border-spacing: 0px;\">\n")

	for y := outputImageBounds.Min.Y; y < outputImageBounds.Max.Y; y++ {
		w.WriteString("<tr")
		if y == 0 { // // draw top bead board horizontal border
			w.WriteString(" class=\"tb\"")
		}
		w.WriteString(">")

		var pixel color.RGBA
		// write a line with colored cells
		for x := outputImageBounds.Min.X; x < outputImageBounds.Max.X; x++ {
			if *beadStyle {
				pixel = outputImage.RGBAAt((x*8)+1, (y*8)+1)
			} else {
				pixel = outputImage.RGBAAt(x, y)
			}
			colorstring := fmt.Sprintf("#%02X%02X%02X", pixel.R, pixel.G, pixel.B)

			w.WriteString("<td bgcolor=\"" + colorstring + "\"")
			if x == 0 {
				w.WriteString(" class=\"lb\"") // draw left bead board vertical border
			} else {
				if (x+1)%*boardDimension == 0 { // draw bead board vertical border
					w.WriteString(" class=\"rb\"")
				}
			}
			w.WriteString(">&nbsp;</td>")
		}
		w.WriteString("</tr>\n")

		w.WriteString("<tr class=\"bg")
		if y > 0 && (y+1)%*boardDimension == 0 { // draw bead board horizontal border
			w.WriteString(" bb")
		}
		w.WriteString("\">")

		// write a line with bead names
		for x := outputImageBounds.Min.X; x < outputImageBounds.Max.X; x++ {
			beadName := outputImageBeadNames[x+y*outputImageBounds.Max.X]
			shortName := strings.Split(beadName, " ")

			w.WriteString("<td")
			if x == 0 {
				w.WriteString(" class=\"lb\"") // draw left bead board vertical border
			} else {
				if (x+1)%*boardDimension == 0 { // draw bead board vertical border
					w.WriteString(" class=\"rb\"")
				}
			}
			w.WriteString(">&nbsp;" + shortName[0] + "&nbsp;</td>") // only print first part of name
		}
		w.WriteString("</tr>\n")
	}

	w.WriteString("</table>\n</body>\n</html>\n")
	w.Flush()
	htmlFile.Close()
}

// FindSimilarColor finds the most similar color from bead palette to the given pixel
func FindSimilarColor(cfgLab map[chromath.Lab]string, pixel color.Color) string {
	colorMatchCacheLock.RLock()
	match, found := colorMatchCache[pixel]
	colorMatchCacheLock.RUnlock()
	if found {
		return match
	}

	rgbLabCacheLock.RLock()
	labPixel, found := rgbLabCache[pixel]
	rgbLabCacheLock.RUnlock()
	if !found {
		r, g, b, _ := pixel.RGBA()
		rgb := chromath.RGB{float64(uint8(r)), float64(uint8(g)), float64(uint8(b))}
		xyz := rgbTransformer.Convert(rgb)
		labPixel = labTransformer.Invert(xyz)
		rgbLabCacheLock.Lock()
		rgbLabCache[pixel] = labPixel
		rgbLabCacheLock.Unlock()
	}

	var bestBeadMatch string
	minDistance := -1.0 // < 0 is uninitialized marker
	for lab, beadName := range cfgLab {
		distance := deltae.CIE2000(lab, labPixel, &deltae.KLChDefault)
		if minDistance < 0.0 || distance < minDistance {
			minDistance = distance
			bestBeadMatch = beadName
		}
		if *verbose == true {
			fmt.Printf("Match: %v with distance: %v\n", beadName, distance)
		}
	}

	if *verbose == true {
		fmt.Printf("Best match: %v with distance: %v\n", bestBeadMatch, minDistance)
	}
	colorMatchCacheLock.Lock()
	colorMatchCache[pixel] = bestBeadMatch
	colorMatchCacheLock.Unlock()
	return bestBeadMatch
}

// LoadPalette loads a palette from a json file and returns a LAB color palette
func LoadPalette(fileName string) (map[string]BeadConfig, map[chromath.Lab]string) {
	cfgData, err := ioutil.ReadFile(fileName)
	if err != nil {
		fmt.Printf("File error: %v\n", err)
		os.Exit(1)
	}

	cfg := make(map[string]BeadConfig)
	err = json.Unmarshal(cfgData, &cfg)
	if err != nil {
		fmt.Printf("Config json error: %v\n", err)
		os.Exit(1)
	}

	cfgLab := make(map[chromath.Lab]string)
	for beadName, rgbOriginal := range cfg {
		if *greyScale == true && rgbOriginal.GreyShade == false { // only process grey shades in greyscale mode
			continue
		}
		if *useTranslucent == false && rgbOriginal.Translucent == true { // only process translucent in translucent mode
			continue
		}
		if *useFlourescent == false && rgbOriginal.Flourescent == true { // only process flourescent in flourescent mode
			continue
		}

		rgb := chromath.RGB{float64(rgbOriginal.R), float64(rgbOriginal.G), float64(rgbOriginal.B)}
		xyz := rgbTransformer.Convert(rgb)
		lab := labTransformer.Invert(xyz)
		cfgLab[lab] = beadName
		if *verbose == true {
			fmt.Printf("Loaded bead: '%v', RGB: %v, Lab: %v\n", beadName, rgb, lab)
		}
	}

	return cfg, cfgLab
}
