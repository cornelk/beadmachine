package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"os"
	"strings"

	"github.com/cornelk/gotokit/log"
	"github.com/jkl1337/go-chromath"
	"github.com/jkl1337/go-chromath/deltae"
)

// writeHTMLBeadInstructionFile writes a HTML file with instructions on how to make the bead based image.
func (m *beadMachine) writeHTMLBeadInstructionFile(outputImageBounds image.Rectangle, outputImage *image.RGBA,
	outputImageBeadNames []string) error {

	htmlFile, err := os.Create(m.htmlFileName)
	if err != nil {
		return fmt.Errorf("creating HTML bead instruction file: %w", err)
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
			if m.beadStyle {
				pixel = outputImage.RGBAAt((x*8)+1, (y*8)+1)
			} else {
				pixel = outputImage.RGBAAt(x, y)
			}
			colorstring := fmt.Sprintf("#%02X%02X%02X", pixel.R, pixel.G, pixel.B)

			w.WriteString("<td bgcolor=\"" + colorstring + "\"")
			if x == 0 {
				w.WriteString(" class=\"lb\"") // draw left bead board vertical border
			} else if (x+1)%m.boardDimension == 0 { // draw bead board vertical border
				w.WriteString(" class=\"rb\"")
			}
			w.WriteString(">&nbsp;</td>")
		}
		w.WriteString("</tr>\n")

		w.WriteString("<tr class=\"bg")
		if y > 0 && (y+1)%m.boardDimension == 0 { // draw bead board horizontal border
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
			} else if (x+1)%m.boardDimension == 0 { // draw bead board vertical border
				w.WriteString(" class=\"rb\"")
			}
			w.WriteString(">&nbsp;" + shortName[0] + "&nbsp;</td>") // only print first part of name
		}
		w.WriteString("</tr>\n")
	}

	w.WriteString("</table>\n</body>\n</html>\n")
	w.Flush()
	htmlFile.Close()
	return nil
}

// findSimilarColor finds the most similar color from bead palette to the given pixel.
func (m *beadMachine) findSimilarColor(cfgLab map[chromath.Lab]string, pixel color.Color) string {
	m.colorMatchCacheLock.RLock()
	match, found := m.colorMatchCache[pixel]
	m.colorMatchCacheLock.RUnlock()
	if found {
		return match
	}

	m.rgbLabCacheLock.RLock()
	labPixel, found := m.rgbLabCache[pixel]
	m.rgbLabCacheLock.RUnlock()
	if !found {
		r, g, b, _ := pixel.RGBA()
		rgb := chromath.RGB{float64(uint8(r)), float64(uint8(g)), float64(uint8(b))}
		xyz := m.rgbTransformer.Convert(rgb)
		labPixel = m.labTransformer.Invert(xyz)
		m.rgbLabCacheLock.Lock()
		m.rgbLabCache[pixel] = labPixel
		m.rgbLabCacheLock.Unlock()
	}

	var bestBeadMatch string
	minDistance := -1.0 // < 0 is uninitialized marker
	for lab, beadName := range cfgLab {
		distance := deltae.CIE2000(lab, labPixel, &deltae.KLChDefault)
		if minDistance < 0.0 || distance < minDistance {
			minDistance = distance
			bestBeadMatch = beadName
		}
		m.logger.Debug("Matched color",
			log.String("bead", beadName),
			log.Float64("distance", distance))
	}

	m.logger.Debug("Best color match",
		log.String("bead", bestBeadMatch),
		log.Float64("distance", minDistance))
	m.colorMatchCacheLock.Lock()
	m.colorMatchCache[pixel] = bestBeadMatch
	m.colorMatchCacheLock.Unlock()
	return bestBeadMatch
}

// loadPalette loads a palette from a json file and returns a LAB color palette.
func (m *beadMachine) loadPalette() (map[string]BeadConfig, map[chromath.Lab]string, error) {
	cfgData, err := os.ReadFile(m.paletteFileName)
	if err != nil {
		return nil, nil, fmt.Errorf("opening palette file: %w", err)
	}

	cfg := make(map[string]BeadConfig)
	if err = json.Unmarshal(cfgData, &cfg); err != nil {
		return nil, nil, fmt.Errorf("unmarshalling palette file: %w", err)
	}

	cfgLab := make(map[chromath.Lab]string)
	for beadName, rgbOriginal := range cfg {
		if m.greyScale && !rgbOriginal.GreyShade { // only process grey shades in greyscale mode
			continue
		}
		if !m.translucent && rgbOriginal.Translucent { // only process translucent in translucent mode
			continue
		}
		if !m.fluorescent && rgbOriginal.fluorescent { // only process fluorescent in fluorescent mode
			continue
		}

		rgb := chromath.RGB{float64(rgbOriginal.R), float64(rgbOriginal.G), float64(rgbOriginal.B)}
		xyz := m.rgbTransformer.Convert(rgb)
		lab := m.labTransformer.Invert(xyz)
		cfgLab[lab] = beadName
		m.logger.Debug("Bead loaded",
			log.String("bead", beadName),
			log.Object("RGB", rgb),
			log.Object("Lab", lab),
		)
	}

	return cfg, cfgLab, nil
}
