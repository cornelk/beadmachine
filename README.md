# beadmachine
beadmachine is a bead pattern creator. Convert any image to a bead pattern that matches the bead colors for every pixel with the best matching bead color. It also generates a statistic about the used beads.

### Features
- Cross platform
- Uses all available cores to process the image
- Supports gif/jpg/png as input file formats
- Color matching based on [CIEDE2000](http://en.wikipedia.org/wiki/Color_difference#CIEDE2000 "")

#### Command-line options:
<dl>
<dt>-i=FILENAME</dt>
  <dd>Filename of image to process.</dd>
<dt>-o=FILENAME.png</dt>
  <dd>Output filename for the converted PNG image.</dd>
<dt>-p=colors_hama.json</dt>
  <dd>Filename of the bead palette.</dd>
</dl>

### Todo
- Image resizing
- Support for giving bead stocks as input
- GUI

